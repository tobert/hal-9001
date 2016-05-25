package hal

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

/* While it's possible to use the standard library flags or an off-the-github
 * command-line parser, they have proven to be clunky and often hacky to use.
 * This API is purpose-built for building bot plugins, focusing on doing the
 * tedious parts of parsing commands without getting in the way.
 * Rules:
 *   1. "*" as user input means "whatever, from the current context" e.g. --room *
 *   2. "*" as a Cmd.Token means "anything and everything remaining in argv"
 */

// supported time formats for ParamInst.Time()
var TimeFormats = [...]string{
	"2006-01-02",
	"2006-01-02-07:00",
	"2006-01-02T15:04",
	"2006-01-02T15:04-07:00",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05-07:00",
}

// Cmd models a tree of commands and subcommands along with their parameters.
// The tree will almost always be 1 or 2 levels deep. Deeper is possible but
// unlikely to be much higher, KISS.
// TODO: switc to maps for (kv|bool|idx)params and maybe subCmds
type Cmd struct {
	token      string // * => slurp everything remaining
	usage      string
	subCmds    []*SubCmd
	kvparams   []*KVParam
	boolparams []*BoolParam
	idxparams  map[int]*IdxParam
	aliases    []string
	prev       *Cmd // parent command, nil for root
	mustSubCmd bool // a subcommand is always required
}

type SubCmd struct {
	cmd *Cmd
	Cmd
}

type CmdInst struct {
	cmd            *Cmd
	subCmdInst     *SubCmdInst
	kvparaminsts   []*KVParamInst
	boolparaminsts []*BoolParamInst
	idxparaminsts  map[int]*IdxParamInst
	remainder      []string // args left over after parsing, usually empty
}

type SubCmdInst struct {
	subCmd *SubCmd
	CmdInst
}

// key/value parameters, e.g. "--foo=bar", "foo=bar", "-f bar", "--foo bar"
type KVParam struct {
	key      string   // the "foo" in --foo, -f, foo=bar
	aliases  []string // parameter aliases, e.g. foo => f
	usage    string   // usage string for generating help
	required bool     // whether or not this parameter is required
	cmd      *Cmd     // the (top-level) command the param is attached to
	subcmd   *SubCmd  // the subcommand the param is attached to
}

// keyed parameters that are boolean (flags), e.g. "--foo", "-f", "foo=true"
// do not change this to type BoolParam KVParm - BoolParam's methods will
// become invisble.
type BoolParam struct {
	KVParam
}

// positional parameters (0 indexed)
type IdxParam struct {
	idx      int // positional arg index
	usage    string
	required bool
	cmd      *Cmd
	subcmd   *SubCmd
}

// KVParamInst represents a key/value parameter found in the command
type KVParamInst struct {
	cmdinst    *CmdInst    // the top-level command
	subcmdinst *SubCmdInst // the subcommand the param belongs to, nil for top-level
	param      *KVParam
	found      bool   // was the parameter set?
	arg        string // the original/unmodified argument (e.g. --foo, -f)
	key        string // the key, e.g. "foo"
	value      string
}

// BoolParamInst represents a flag/boolean parameter found in the command
type BoolParamInst struct {
	cmdinst    *CmdInst
	subcmdinst *SubCmdInst
	param      *BoolParam
	found      bool
	arg        string
	key        string
	value      bool
}

// IdxParamInst represents a positional parameter found in the command
type IdxParamInst struct {
	cmdinst    *CmdInst
	subcmdinst *SubCmdInst
	param      *IdxParam
	found      bool
	idx        int
	value      string
}

// tmpParamInst used by the parser to hold keyed parameters before attaching to commands/subcommands.
type tmpParamInst struct {
	cmd        *Cmd
	cmdinst    *CmdInst
	subcmd     *SubCmd
	subcmdinst *SubCmdInst
	found      bool
	arg        string
	key        string
	value      string
}

type stringValuedParamInst interface {
	Found() bool
	Required() bool
	Value() string
	String() (string, error)
	Int() (int, error)
	Float() (float64, error)
	Bool() (bool, error)
	errParam() interface{}
}

// cmdorsubcmd is used internally to pass either a Cmd or SubCmd
// to a helper function so I don't have to copy/paste the code
type cmdorsubcmd interface {
	HasKVParam(string) bool
	HasBoolParam(string) bool
	HasIdxParam(int) bool
	GetKVParam(string) *KVParam
	GetBoolParam(string) *BoolParam
	GetIdxParam(int) *IdxParam
	appendKVParamInst(*KVParamInst)
	appendBoolParamInst(*BoolParamInst)
	appendIdxParamInst(*IdxParamInst)
}

// RequiredParamNotFound is returned when a parameter has Required=true
// and a method was used to access the value but no value was set in the
// command.
type RequiredParamNotFound struct {
	Param interface{}
}

// Error fulfills the Error interface.
func (e RequiredParamNotFound) Error() string {
	name := "BUG(unknown)"

	switch e.Param.(type) {
	case KVParam:
		name = e.Param.(KVParam).key
	case BoolParam:
		name = e.Param.(BoolParam).key
	case IdxParam:
		name = strconv.Itoa(e.Param.(IdxParam).idx)
	}

	return fmt.Sprintf("Parameter %q is required but not set.", name)
}

// UnsupportedTimeFormatError is returned when a provided time string cannot
// be parsed with one of the pre-defined time formats.
type UnsupportedTimeFormatError struct {
	value string
}

// Error fulfills the Error interface for UnsupportedTimeFormatError.
func (e UnsupportedTimeFormatError) Error() string {
	return fmt.Sprintf("Time string %q does not appear to be in a supported format.", e.value)
}

// NewCmd returns an initialized Cmd.
func NewCmd(token string, mustsubcmd bool) *Cmd {
	cmd := Cmd{token: token, mustSubCmd: mustsubcmd}
	return &cmd
}

// ListSubCmds makes sure the SubCmds list is initialized and returns the list.
func (c *Cmd) ListSubCmds() []*SubCmd {
	if c.subCmds == nil {
		c.subCmds = make([]*SubCmd, 0)
	}

	return c.subCmds
}

// _kvparams makes sure the _kvparams list is initialized and returns the list.
func (c *Cmd) _kvparams() []*KVParam {
	if c.kvparams == nil {
		c.kvparams = make([]*KVParam, 0)
	}

	return c.kvparams
}

// _boolparams makes sure the _boolparams list is initialized and returns the list.
func (c *Cmd) _boolparams() []*BoolParam {
	if c.boolparams == nil {
		c.boolparams = make([]*BoolParam, 0)
	}

	return c.boolparams
}

// _idxparams makes sure the _idxparams map is initialized and returns the map.
func (c *Cmd) _idxparams() map[int]*IdxParam {
	if c.idxparams == nil {
		c.idxparams = make(map[int]*IdxParam)
	}

	return c.idxparams
}

// Aliases makes sure the Aliases list is initialized and returns the list.
func (c *Cmd) Aliases() []string {
	if c.aliases == nil {
		c.aliases = make([]string, 0)
	}

	return c.aliases
}

// assertZeroIdxParams panics if there are any IdxParam defined.
func (c *Cmd) assertZeroIdxParams() {
	pps := c._idxparams()
	if len(pps) > 0 {
		log.Panic("Illegal mixing of positional and key/value parameters.")
	}
}

// assertZeroKeyParams panics if there are any BoolParam or KVParam defined.
func (c *Cmd) assertZeroKeyParams() {
	kps := c._kvparams()
	bps := c._boolparams()
	if len(kps) > 0 || len(bps) > 0 {
		log.Panic("Illegal mixing of positional and key/value parameters.")
	}
}

// AddKVParam creates and adds a key/value parameter to the command handle
// and returns the new parameter.
func (c *Cmd) AddKVParam(key string, required bool) *KVParam {
	c.assertZeroIdxParams()

	p := KVParam{key: key}
	p.required = required
	p.cmd = c.Cmd()

	c.kvparams = append(c._kvparams(), &p)

	return &p
}

// AddBoolParam adds a boolean/flag parameter to the command and returns the
// new parameter.
func (c *Cmd) AddBoolParam(key string, required bool) *BoolParam {
	c.assertZeroIdxParams()

	p := BoolParam{}
	p.key = key
	p.required = required
	p.cmd = c.Cmd()

	c.boolparams = append(c._boolparams(), &p)

	return &p
}

// AddIdxParam adds a positional parameter to the command and returns the
// new parameter.
func (c *Cmd) AddIdxParam(position int, required bool) *IdxParam {
	c.assertZeroKeyParams()

	ips := c._idxparams()

	if _, exists := ips[position]; exists {
		log.Panicf("position %d already has an IdxParam defined on this command", position)
	}

	p := IdxParam{idx: position}
	p.required = required
	p.cmd = c.Cmd()

	ips[position] = &p

	return &p
}

// AddKVParam creates and adds a key/value parameter to the subcommand
// and returns the new parameter.
func (c *SubCmd) AddKVParam(key string, required bool) *KVParam {
	c.assertZeroIdxParams()

	p := KVParam{key: key}
	p.required = required
	p.cmd = c.cmd
	p.subcmd = c

	c.kvparams = append(c._kvparams(), &p)

	return &p
}

// AddBoolParam adds a boolean/flag parameter to the subcommand and returns the
// new parameter.
func (c *SubCmd) AddBoolParam(key string, required bool) *BoolParam {
	c.assertZeroIdxParams()

	p := BoolParam{}
	p.key = key
	p.required = required
	p.cmd = c.cmd
	p.subcmd = c

	c.boolparams = append(c._boolparams(), &p)

	return &p
}

// AddIdxParam adds a positional parameter to the subcommand and returns the
// new parameter.
func (c *SubCmd) AddIdxParam(position int, required bool) *IdxParam {
	c.assertZeroKeyParams()

	ips := c._idxparams()

	if _, exists := ips[position]; exists {
		log.Panicf("position %d already has an IdxParam defined on this subcommand", position)
	}

	p := IdxParam{idx: position}
	p.required = required
	p.cmd = c.cmd
	p.subcmd = c

	ips[position] = &p

	return &p
}

// AddAlias adds an alias to the command and returns the paramter.
func (c *Cmd) AddAlias(alias string) *Cmd {
	c.aliases = append(c.Aliases(), alias)
	return c
}

func (s *SubCmd) AddAlias(alias string) *SubCmd {
	s.aliases = append(s.Aliases(), alias)
	return s
}

// AddAlias adds an alias to the parameter and returns the paramter.
func (p *KVParam) AddAlias(alias string) *KVParam {
	p.aliases = append(p.Aliases(), alias)
	return p
}

func (c *Cmd) Parent() *Cmd {
	return c.prev
}

// MustSubCmd returns bool indicating if a subcommand is required.
func (c *Cmd) MustSubCmd() bool {
	return c.mustSubCmd
}

// Usage returns the auto-generated usage string.
func (c *Cmd) Usage() string {
	return "not implemented yet"
}

// SetUsage sets the usage string for the command. Returns the command.
func (c *Cmd) SetUsage(usage string) *Cmd {
	c.usage = usage
	return c
}

// SetUsage sets the subcommand's usage string.
func (s *SubCmd) SetUsage(usage string) *SubCmd {
	s.usage = usage
	return s
}

// Usage returns the auto-generated usage string for the Command Instance.
func (c *CmdInst) Usage() string {
	return c.cmd.Usage()
}

func (p *KVParam) Usage() string {
	return p.usage
}

func (p *BoolParam) Usage() string {
	return p.usage
}

func (p *IdxParam) Usage() string {
	return p.usage
}

// SetUsage sets the usage string for the paremeter. Returns the parameter.
func (p *KVParam) SetUsage(usage string) *KVParam {
	p.usage = usage
	return p
}

// SetUsage sets the usage string for the paremeter. Returns the parameter.
func (p *BoolParam) SetUsage(usage string) *BoolParam {
	p.usage = usage
	return p
}

// SetUsage sets the usage string for the paremeter. Returns the parameter.
func (p *IdxParam) SetUsage(usage string) *IdxParam {
	p.usage = usage
	return p
}

func (p *KVParam) Key() string {
	return p.key
}

func (p *BoolParam) Key() string {
	return p.key
}

func (p *IdxParam) Idx() int {
	return p.idx
}

func (p *KVParamInst) Key() string {
	return p.key
}

func (p *BoolParamInst) Key() string {
	return p.key
}

func (p *IdxParamInst) Idx() int {
	return p.idx
}

// Cmd returns the command the parameter belongs to. Panics if no command is attached.
func (p *KVParam) Cmd() *Cmd {
	if p.cmd == nil {
		panic("Can't call Cmd() on this KVParam because it is not attached to a Cmd!")
	}

	return p.cmd
}

// Cmd returns the command the parameter belongs to. Panics if no command is attached.
func (p *BoolParam) Cmd() *Cmd {
	if p.cmd == nil {
		panic("Can't call Cmd() on this BoolParam because it is not attached to a Cmd!")
	}

	return p.cmd
}

// Cmd returns the command the parameter belongs to. Panics if no command is attached.
func (p *IdxParam) Cmd() *Cmd {
	if p.cmd == nil {
		panic("Can't call Cmd() on this IdxParam because it is not attached to a Cmd!")
	}

	return p.cmd
}

func (p *KVParam) SubCmd() *SubCmd {
	if p.subcmd == nil {
		panic("Can't call SubCmd() on this KVParam because it is not attached to a SubCmd!")
	}

	return p.subcmd
}

// Cmd returns the command the parameter belongs to. Panics if no command is attached.
func (p *KVParamInst) Cmd() *Cmd {
	if p.param == nil {
		panic("Can't call Cmd() on this KVParamInst because it is not attached to a KVParam!")
	}

	return p.param.Cmd()
}

// Cmd returns the command the parameter belongs to. Panics if no command is attached.
func (p *BoolParamInst) Cmd() *Cmd {
	if p.param == nil {
		panic("Can't call Cmd() on this BoolParamInst because it is not attached to a BoolPararm!")
	}

	return p.param.Cmd()
}

// Cmd returns the command the parameter belongs to. Panics if no command is attached.
func (p *IdxParamInst) Cmd() *Cmd {
	if p.param == nil {
		panic("Can't call Cmd() on this IdxParamInst because it is not attached to a IdxParam!")
	}

	return p.param.Cmd()
}

func (p *KVParamInst) SubCmdInst() *SubCmdInst {
	if p.subcmdinst == nil {
		panic("Can't call SubCmdInst() on this KVParamInst because it is not attached to a SubCmdInst!")
	}

	return p.subcmdinst
}

func (p *BoolParamInst) SubCmdInst() *SubCmdInst {
	if p.subcmdinst == nil {
		panic("Can't call SubCmdInst() on this BoolParamInst because it is not attached to a SubCmdInst!")
	}

	return p.subcmdinst
}

func (p *IdxParamInst) SubCmdInst() *SubCmdInst {
	if p.subcmdinst == nil {
		panic("Can't call SubCmdInst() on this IdxParamInst because it is not attached to a SubCmd!")
	}

	return p.subcmdinst
}

func (p *KVParamInst) Found() bool {
	return p.found
}

func (p *BoolParamInst) Found() bool {
	return p.found
}

func (p *IdxParamInst) Found() bool {
	return p.found
}

func (p *KVParamInst) Required() bool {
	return p.param.required
}

func (p *BoolParamInst) Required() bool {
	return p.param.required
}

func (p *IdxParamInst) Required() bool {
	return p.param.required
}

func (p *KVParamInst) Param() *KVParam {
	return p.param
}

func (p *BoolParamInst) Param() *BoolParam {
	return p.param
}

func (p *IdxParamInst) Param() *IdxParam {
	return p.param
}

// errParam is used to get an interface{} handle to return in errors.
// See: RequiredParamNotFound
func (p *KVParamInst) errParam() interface{} {
	return p.param
}

// errParam is used to get an interface{} handle to return in errors.
func (p *BoolParamInst) errParam() interface{} {
	return p.param
}

// errParam is used to get an interface{} handle to return in errors.
func (p *IdxParamInst) errParam() interface{} {
	return p.param
}

// Cmd returns the command it was called on. It does nothing and exists to
// make it possible to format chained calls nicely.
func (c *Cmd) Cmd() *Cmd {
	return c
}

func (s *SubCmd) SubCmd() *SubCmd {
	return s
}

func (c *Cmd) Token() string {
	return c.token
}

// AddCmd adds a subcommand to the handle and returns the new (sub-)command.
func (c *Cmd) AddSubCmd(token string) *SubCmd {
	sub := SubCmd{}
	sub.prev = c
	sub.token = token

	c.subCmds = append(c.ListSubCmds(), &sub)

	return &sub
}

func (c *Cmd) GetKVParam(key string) *KVParam {
	for _, p := range c._kvparams() {
		if p.key == key {
			return p
		}
	}

	return nil
}

func (c *Cmd) GetBoolParam(key string) *BoolParam {
	for _, p := range c._boolparams() {
		if p.key == key {
			return p
		}
	}

	return nil
}

// GetIdxParam gets a positional parameter by its index.
func (c *Cmd) GetIdxParam(idx int) *IdxParam {
	ips := c._idxparams()

	if p, exists := ips[idx]; exists {
		return p
	}

	panic("No parameter available.")
}

func (c *Cmd) HasKVParam(key string) bool {
	for _, p := range c._kvparams() {
		if p.key == key {
			return true
		}
	}

	return false
}

func (c *Cmd) HasBoolParam(key string) bool {
	for _, p := range c._boolparams() {
		if p.key == key {
			return true
		}
	}

	return false
}

func (c *Cmd) HasIdxParam(idx int) bool {
	ips := c._idxparams()
	_, exists := ips[idx]
	return exists
}

func (c *Cmd) SubCmds() []*SubCmd {
	return c.ListSubCmds()
}

// GetSubCmd gets a subcommand by its token. Returns nil for no match.
func (c *Cmd) GetSubCmd(token string) *SubCmd {
	for _, s := range c.ListSubCmds() {
		if s.token == token {
			return s
		}
	}

	return nil
}

// parse a list of argv-style strings (0 is always the command name e.g. []string{"prefs"})
// foo bar --baz
// foo --bar baz --version
// foo bar=baz
// foo x=y z=q init --foo baz
// TODO: automatic emdash cleanup
// TODO: enforce MustSubCmd
// TODO: return errors instead of nil/panic
func (c *Cmd) Process(argv []string) *CmdInst {
	// a hand-coded argument processor that evaluates the provided argv list
	// against the command definition and returns a CmdInst with all of the
	// available data parsed and ready to use with CmdInst/ParamInst methods.

	// the top-level command instance
	topInst := CmdInst{cmd: c}

	// no arguments were provided
	if len(argv) == 1 {
		return &topInst
	}

	var curSubCmdInst *SubCmdInst // the current subcommand - changes during parsing
	var curSubCmdIdx int          // the idx the subcommand found in argv
	var skipNext bool
	var looseParams []*tmpParamInst

	// first pass: extract subcommands and parameters
	for i, arg := range argv[1:] {
		if skipNext {
			skipNext = false
			continue
		}

		var key, value, next string
		var nextExists bool

		if i+2 < len(argv) {
			next = argv[i+2]
			nextExists = true
		} else {
			nextExists = false
		}

		if c.HasIdxParam(i - 1) {
			// top-level command has positional parameters
			pi := IdxParamInst{
				cmdinst: &topInst,
				found:   true,
				idx:     i - 1,
				param:   c.GetIdxParam(i - 1),
				value:   arg,
			}

			topInst.appendIdxParamInst(&pi)
		} else if curSubCmdInst != nil && curSubCmdInst.HasIdxParam(0) {
			// subcommand has positional parameters
			paramIdx := i - curSubCmdIdx - 1

			pi := IdxParamInst{
				cmdinst:    &topInst,
				subcmdinst: curSubCmdInst,
				found:      true,
				idx:        paramIdx,
				param:      curSubCmdInst.GetIdxParam(paramIdx),
				value:      arg,
			}

			curSubCmdInst.appendIdxParamInst(&pi)
		} else if strings.Contains(arg, "=") {
			// looks like a key=value or --key=value parameter
			// could be --foo=bar but all that matters is the "foo"
			// could be --foo=true for BoolParam and that's fine too
			kv := strings.SplitN(arg, "=", 2)
			key = strings.TrimLeft(kv[0], "-")
			value = kv[1]
			// falls through, further processing below this if block...
		} else if looksLikeParam(arg) {
			// looks like a parameter
			// e.g. --foo bar -f bar
			key = strings.TrimLeft(arg, "-")
			if nextExists && !looksLikeParam(next) {
				value = next
				skipNext = true
			}
			// falls through, further processing below this if block...
		} else if curSubCmdInst == nil && c.HasSubCmdToken(arg) {
			// the first subcommand - the "foo" in "!command foo bar --baz"
			for _, sc := range topInst.cmd.ListSubCmds() {
				if sc.token == arg {
					sci := SubCmdInst{subCmd: sc}
					sci.cmd = c
					curSubCmdInst = &sci
					topInst.subCmdInst = &sci
					break
				}
			}

			continue // processed a subcommand, move onto the next arg
		} else if curSubCmdInst != nil && curSubCmdInst.subCmd.HasSubCmdToken(arg) {
			// sub-subcommands - the "bar" or "blargh" in "!command foo bar blargh --baz"
			for _, sc := range curSubCmdInst.subCmd.ListSubCmds() {
				if arg == sc.token {
					sci := SubCmdInst{subCmd: sc}
					sci.cmd = c

					// point the current subcommand to the new one
					curSubCmdInst.subCmdInst = &sci

					// advance "current" to the new subcommand
					curSubCmdInst = &sci

					// set the index where the subcommand was discovered for use
					// in extracting postitional parameters (above)
					curSubCmdIdx = i
				}
			}

			continue // processed a subcommand, move onto the next arg
		} else {
			// leftover/unrecognized args go in .remainder
			topInst.remainder = append(topInst.Remainder(), arg)
			continue
		}

		pinst := tmpParamInst{}
		pinst.key = key
		pinst.arg = arg
		pinst.value = value
		pinst.found = true
		pinst.cmd = c
		pinst.cmdinst = &topInst

		// subcommands get the first shot at a parameter
		// !foo --bar baz --bar
		// !foo baz --bar
		// !foo --bar baz
		if curSubCmdInst != nil && curSubCmdInst.subCmd.HasKeyParam(key) {
			// the parameter belongs to the subcommand
			pinst.subcmd = curSubCmdInst.subCmd
			pinst.subcmdinst = curSubCmdInst
			pinst.attachKeyParam(curSubCmdInst)
		} else if c.HasKeyParam(key) {
			// the parameter belongs to the command
			pinst.attachKeyParam(&topInst)
		} else {
			// store (likely) out-of-order parameters to process after all args &
			// subcommands are discovered
			looseParams = append(looseParams, &pinst)
		}
	}

	// find a home for out-of-order parameters, panic if that fails since it's a bug
	for _, linst := range looseParams {
		if topInst.subCmdInst == nil {
			panic("found out-of-order params but no subcommand! Maybe bug, maybe I need to put a better error here...")
		}
		linst.findAndAttachKeyParam(topInst.subCmdInst)
	}

	return &topInst
}

// looksLikeBool checks to see if the provided value contains "true" or "false"
// in any case combination.
func looksLikeBool(val string) bool {
	lcval := strings.ToLower(val)

	if strings.Contains(lcval, "true") {
		return true
	}

	if strings.Contains(lcval, "false") {
		return true
	}

	return false
}

// looksLikeParam returns true if there is a leading - or an = in the string.
func looksLikeParam(key string) bool {
	if strings.HasPrefix(key, "-") {
		return true
	} else if strings.Contains(key, "=") {
		return true
	} else {
		return false
	}
}

func (tmp *tmpParamInst) attachKeyParam(whatever cmdorsubcmd) {
	if whatever.HasKVParam(tmp.key) {
		p := whatever.GetKVParam(tmp.key)
		pi := KVParamInst{
			arg:        tmp.arg,
			cmdinst:    tmp.cmdinst,
			found:      tmp.found,
			key:        tmp.key,
			param:      p,
			subcmdinst: tmp.subcmdinst,
			value:      tmp.value,
		}

		switch whatever.(type) {
		case *CmdInst:
			ci := whatever.(*CmdInst)
			ci.kvparaminsts = append(ci.ListKVParamInsts(), &pi)
		case *SubCmdInst:
			sci := whatever.(*SubCmdInst)
			sci.kvparaminsts = append(sci.ListKVParamInsts(), &pi)
		}
	} else if whatever.HasBoolParam(tmp.key) {
		val, err := strconv.ParseBool(tmp.value)
		if err != nil {
			log.Panicf("invalid bool value %q for key %q", tmp.value, tmp.key)
		}

		p := whatever.GetBoolParam(tmp.key)
		pi := BoolParamInst{
			arg:        tmp.arg,
			cmdinst:    tmp.cmdinst,
			found:      tmp.found,
			key:        tmp.key,
			param:      p,
			subcmdinst: tmp.subcmdinst,
			value:      val,
		}

		switch whatever.(type) {
		case *CmdInst:
			ci := whatever.(*CmdInst)
			ci.boolparaminsts = append(ci.ListBoolParamInsts(), &pi)
		case *SubCmdInst:
			sci := whatever.(*SubCmdInst)
			sci.boolparaminsts = append(sci.ListBoolParamInsts(), &pi)
		}
	} else {
		log.Panicf("BUG: arg %q does not have a matching parameter for key %q", tmp.arg, tmp.key)
	}
}

func (tmp *tmpParamInst) findAndAttachKeyParam(sub *SubCmdInst) {
	if sub.HasBoolParam(tmp.key) || sub.HasKVParam(tmp.key) {
		tmp.attachKeyParam(sub)
	} else if sub.subCmdInst != nil {
		tmp.findAndAttachKeyParam(sub.subCmdInst)
	}
}

// HasSubCmdToken returns whether or not the proivded token is defined as a subcommand.
func (c *Cmd) HasSubCmdToken(token string) bool {
	if c == nil {
		return false
	}

	for _, sc := range c.ListSubCmds() {
		if token == sc.token {
			return true
		}
	}

	return false
}

// HasKeyParam returns true if there are any parameters defined with
// the provided key of either key type (bool or kv).
func (c *Cmd) HasKeyParam(key string) bool {
	if c == nil {
		return false
	}

	for _, p := range c._boolparams() {
		if key == p.key {
			return true
		}
	}

	for _, p := range c._kvparams() {
		if key == p.key {
			return true
		}
	}

	return false
}

// SubCmdToken returns the subcommand's token string. Returns empty string
// if there is no subcommand.
func (c *CmdInst) SubCmdToken() string {
	if c.subCmdInst != nil {
		return c.subCmdInst.subCmd.token
	}

	return ""
}

func (c *SubCmdInst) SubCmdToken() string {
	if c.subCmdInst != nil {
		return c.subCmdInst.subCmd.token
	}

	return ""
}

func (c *CmdInst) SubCmdInst() *SubCmdInst {
	return c.subCmdInst
}

func (c *CmdInst) HasKVParamInst(key string) bool {
	for _, p := range c.ListKVParamInsts() {
		if p.key == key {
			return true
		}
	}

	return false
}

func (c *CmdInst) HasKVParam(key string) bool {
	return c.cmd.HasKVParam(key)
}

func (c *SubCmdInst) HasKVParam(key string) bool {
	return c.subCmd.HasKVParam(key)
}

func (c *CmdInst) HasBoolParamInst(key string) bool {
	for _, p := range c.ListBoolParamInsts() {
		if p.key == key {
			return true
		}
	}

	return false
}

func (c *CmdInst) HasBoolParam(key string) bool {
	return c.cmd.HasBoolParam(key)
}

func (c *CmdInst) HasIdxParamInst(idx int) bool {
	ipis := c.mapIdxParamInsts()
	_, exists := ipis[idx]
	return exists
}

func (c *CmdInst) HasIdxParam(idx int) bool {
	return c.cmd.HasIdxParam(idx)
}

func (c *SubCmdInst) HasIdxParam(idx int) bool {
	return c.subCmd.HasIdxParam(idx)
}

// GetKVParamInst gets a key/value parameter instance by its key.
func (c *CmdInst) GetKVParamInst(key string) *KVParamInst {
	for _, p := range c.ListKVParamInsts() {
		if p.key == key {
			return p
		}
	}

	// TODO: decide if this is the right thing to do
	log.Panicf("GetKVParamInst(%q) failed to find an entry. Did you test with HasKVParamInst first?", key)

	return nil
}

func (c *CmdInst) GetKVParam(key string) *KVParam {
	for _, p := range c.cmd._kvparams() {
		if p.key == key {
			return p
		}
	}

	return nil
}

func (c *SubCmdInst) GetKVParam(key string) *KVParam {
	for _, p := range c.subCmd._kvparams() {
		if p.key == key {
			return p
		}
	}

	return nil
}

// GetBoolParamInst gets a key/value parameter instance by its key.
func (c *CmdInst) GetBoolParamInst(key string) *BoolParamInst {
	for _, p := range c.ListBoolParamInsts() {
		if p.key == key {
			return p
		}
	}

	return nil
}

func (c *CmdInst) GetBoolParam(key string) *BoolParam {
	for _, p := range c.cmd._boolparams() {
		if p.key == key {
			return p
		}
	}

	return nil
}

func (c *SubCmdInst) GetBoolParam(key string) *BoolParam {
	for _, p := range c.subCmd._boolparams() {
		if p.key == key {
			return p
		}
	}

	return nil
}

// GetIdxParamInst gets a positional parameter instance by its index.
func (c *CmdInst) GetIdxParamInst(idx int) *IdxParamInst {
	ipis := c.mapIdxParamInsts()
	if p, exists := ipis[idx]; exists {
		return p
	}

	panic("No parameter instance.")
}

func (c *CmdInst) GetIdxParam(idx int) *IdxParam {
	ips := c.cmd._idxparams()
	if p, exists := ips[idx]; exists {
		return p
	}

	return nil
}

func (c *SubCmdInst) GetIdxParam(idx int) *IdxParam {
	ips := c.subCmd._idxparams()
	if p, exists := ips[idx]; exists {
		return p
	}

	return nil
}

func (c *CmdInst) appendKVParamInst(pi *KVParamInst) {
	c.kvparaminsts = append(c.ListKVParamInsts(), pi)
}

func (c *CmdInst) appendBoolParamInst(pi *BoolParamInst) {
	c.boolparaminsts = append(c.ListBoolParamInsts(), pi)
}

func (c *CmdInst) appendIdxParamInst(pi *IdxParamInst) {
	ipis := c.mapIdxParamInsts()
	ipis[pi.idx] = pi
}

// ListKVParamInsts initializes the kvparaminsts list on the fly and returns it.
func (c *CmdInst) ListKVParamInsts() []*KVParamInst {
	if c.kvparaminsts == nil {
		c.kvparaminsts = make([]*KVParamInst, 0)
	}

	return c.kvparaminsts
}

// ListBoolParamInsts initializes the boolparaminsts list on the fly and returns it.
func (c *CmdInst) ListBoolParamInsts() []*BoolParamInst {
	if c.boolparaminsts == nil {
		c.boolparaminsts = make([]*BoolParamInst, 0)
	}

	return c.boolparaminsts
}

// mapIdxParamInsts initializes the idxparaminsts list on the fly and returns it.
func (c *CmdInst) mapIdxParamInsts() map[int]*IdxParamInst {
	if c.idxparaminsts == nil {
		c.idxparaminsts = make(map[int]*IdxParamInst)
	}

	return c.idxparaminsts
}

func (c *CmdInst) ListIdxParamInsts() []*IdxParamInst {
	ipis := c.mapIdxParamInsts()
	out := make([]*IdxParamInst, len(ipis))

	for i, pi := range ipis {
		out[i] = pi
	}

	return out
}

// Remainder initializes the remainder list on the fly and returns it.
func (c *CmdInst) Remainder() []string {
	if c.remainder == nil {
		c.remainder = make([]string, 0)
	}

	return c.remainder
}

// Aliases initializes the aliases list on the fly and returns it.
func (p *KVParam) Aliases() []string {
	if p.aliases == nil {
		p.aliases = make([]string, 0)
	}

	return p.aliases
}

func (p *KVParamInst) Value() string {
	return p.value
}

func (p *BoolParamInst) Value() bool {
	return p.value
}

func (p *IdxParamInst) Value() string {
	return p.value
}

// String returns the value as a string.
func (p *KVParamInst) String() (string, error) {
	if !p.found && p.param.required {
		return "", RequiredParamNotFound{p.param}
	}

	return p.value, nil
}

// String returns the value as a string.
func (p *BoolParamInst) String() (string, error) {
	if !p.found && p.param.required {
		return "", RequiredParamNotFound{p.param}
	}

	if p.value {
		return "true", nil
	} else {
		return "false", nil
	}
}

// String returns the value as a string.
func (p *IdxParamInst) String() (string, error) {
	if !p.found && p.param.required {
		return "", RequiredParamNotFound{p.param}
	}

	return p.value, nil
}

// String returns the value as an int. If the param is required and it was
// not set, RequiredParamNotFound is returned. Additionally, any errors in
// conversion are returned.
func intParam(p stringValuedParamInst) (int, error) {
	if !p.Found() {
		if p.Required() {
			return 0, RequiredParamNotFound{p.errParam()}
		} else {
			return 0, nil
		}
	}

	val, err := strconv.ParseInt(p.Value(), 10, 64)
	return int(val), err // warning: doesn't handle overflow
}

func (p *KVParamInst) Int() (int, error) {
	return intParam(p)
}

func (p *IdxParamInst) Int() (int, error) {
	return intParam(p)
}

// Float returns the value of the parameter as a float. If the value cannot
// be converted, an error will be returned. See: strconv.ParseFloat
func floatParam(p stringValuedParamInst) (float64, error) {
	if !p.Found() {
		if p.Required() {
			return 0, RequiredParamNotFound{p.errParam()}
		} else {
			return 0, nil
		}
	}

	return strconv.ParseFloat(p.Value(), 64)
}

func (p *KVParamInst) Float() (float64, error) {
	return floatParam(p)
}

func (p *IdxParamInst) Float() (float64, error) {
	return floatParam(p)
}

// Bool returns the value of the parameter as a bool.
// If the value is required and not set, returns RequiredParamNotFound.
// If the value cannot be converted, an error will be returned.
// See: strconv.ParseBool
func boolParam(p stringValuedParamInst) (bool, error) {
	if !p.Found() {
		if p.Required() {
			return false, RequiredParamNotFound{p.errParam()}
		} else {
			return false, nil
		}
	}

	stripped := strings.Trim(p.Value(), `'"`)
	return strconv.ParseBool(stripped)
}

func (p *KVParamInst) Bool() (bool, error) {
	return boolParam(p)
}

func (p *IdxParamInst) Bool() (bool, error) {
	return boolParam(p)
}

// Duration returns the value of the parameter as a Go time.Duration.
// Day and Week (e.g. "1w", "1d") are converted to 168 and 24 hours respectively.
// If the value is required and not set, returns RequiredParamNotFound.
// If the value cannot be converted, an error will be returned.
// See: time.ParseDuration
func durationParam(p stringValuedParamInst) (time.Duration, error) {
	duration := p.Value()
	empty := time.Duration(0)

	if !p.Found() {
		if p.Required() {
			return empty, RequiredParamNotFound{p.errParam()}
		} else {
			return empty, nil
		}
	}

	if strings.HasSuffix(duration, "w") {
		weeks, err := strconv.Atoi(strings.TrimSuffix(duration, "w"))
		if err != nil {
			return empty, fmt.Errorf("Could not convert duration %q: %s", duration, err)
		}

		return time.Hour * time.Duration(weeks*24*7), nil
	} else if strings.HasSuffix(duration, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(duration, "d"))
		if err != nil {
			return empty, fmt.Errorf("Could not convert duration %q: %s", duration, err)
		}
		return time.Hour * time.Duration(days*24), nil
	} else {
		return time.ParseDuration(duration)
	}
}

func (p *KVParamInst) Duration() (time.Duration, error) {
	return durationParam(p)
}

func (p *IdxParamInst) Duration() (time.Duration, error) {
	return durationParam(p)
}

// Time returns the value of the parameter as a Go time.Time.
// Many formats are attempted before giving up.
// If the value is required and not set, returns RequiredParamNotFound.
// If the value cannot be converted, an error will be returned.
// See: TimeFormats in this package
// See: time.ParseDuration
func timeParam(p stringValuedParamInst) (time.Time, error) {
	if !p.Found() {
		if p.Required() {
			return time.Time{}, RequiredParamNotFound{p.errParam()}
		} else {
			return time.Time{}, nil
		}
	}

	t := p.Value()

	// convert Z suffix to +00:00
	if strings.HasSuffix(t, "Z") {
		t = strings.TrimSuffix(t, "Z") + "+00:00"
	}

	// try all of the formats
	for _, fmt := range TimeFormats {
		out, err := time.Parse(fmt, t)
		if err != nil {
			continue
		} else {
			return out, nil
		}
	}

	return time.Time{}, UnsupportedTimeFormatError{t}
}

func (p *KVParamInst) Time() (time.Time, error) {
	return timeParam(p)
}

func (p *IdxParamInst) Time() (time.Time, error) {
	return timeParam(p)
}

// MustString returns the value as a string. If it was required/not-set,
// panic ensues. Empty string may be returned for not-required+not-set.
func (p *KVParamInst) MustString() string {
	out, err := p.String()
	if p.Required() && err != nil {
		panic(err)
	}

	return out
}

func (p *IdxParamInst) MustString() string {
	out, err := p.String()
	if p.Required() && err != nil {
		panic(err)
	}

	return out
}

// DefString returns the value as a string. Rules:
// If the param is required and it was not set, return the provided default.
// If the param is not required and it was not set, return the empty string.
// If the param is set and the value is "*", return the provided default.
// If the param is set, return the value.
func defStringParam(p stringValuedParamInst, def string) string {
	if !p.Found() {
		if p.Required() {
			// not set, required
			return def
		} else {
			// not set, not required
			return ""
		}
	} else if p.Value() == "*" {
		return def
	}

	out, err := p.String()
	if err != nil {
		return def
	}
	return out
}

func (p *KVParamInst) DefString(def string) string {
	return defStringParam(p, def)
}

func (p *IdxParamInst) DefString(def string) string {
	return defStringParam(p, def)
}

// DefInt returns the value as an int. See DefString for the rules.
func defIntParam(p stringValuedParamInst, def int) int {
	if !p.Found() {
		if p.Required() {
			return def
		} else {
			return 0
		}
	} else if p.Value() == "*" {
		return def
	}

	out, err := p.Int()
	if err != nil {
		return def
	}
	return out
}

func (p *KVParamInst) DefInt(def int) int {
	return defIntParam(p, def)
}

func (p *IdxParamInst) DefInt(def int) int {
	return defIntParam(p, def)
}

// DefFloat returns the value as a float. See DefString for the rules.
func defFloatParam(p stringValuedParamInst, def float64) float64 {
	if !p.Found() {
		if p.Required() {
			return def
		} else {
			return 0
		}
	} else if p.Value() == "*" {
		return def
	}

	out, err := p.Float()
	if err != nil {
		return def
	}
	return out
}

// DefBool returns the value as a bool. See DefString for the rules.
func defBoolParam(p stringValuedParamInst, def bool) bool {
	if !p.Found() {
		if p.Required() {
			return def
		} else {
			return false
		}
	} else if p.Value() == "*" {
		return def
	}

	out, err := p.Bool()
	if err != nil {
		return def
	}
	return out
}
