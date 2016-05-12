package hal

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/* hal.Cmd, hal.Param, hal.CmdInst, and hal.ParamInst are handles for hal's
 * command parsing. While it's possible to use the standard library flags or an
 * off-the-github command-line parser, they have proven to be clunky and often
 * hacky to use. This API is purpose-built for building bot plugins, where folks
 * expect a little more flexibility and the ability to use events as context.
 * Rules (as they form)!
 *   1. Cmd and Param are parsed in the order they were defined.
 *   2a. "*" as user input means "whatever, from the current context" e.g. --room *
 *   2b. "*" as a Cmd.Token means "anything and everything remaining in argv"
 */

// common REs for Param.ValidRE
const IntRE = `^\d+$`
const FloatRE = `^(\d+|\d+.\d+)$`
const BoolRE = `(?i:^(true|false)$)`
const CmdRE = `^!\w+$`
const SubCmdRE = `^\w+$`

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
//
// Key is the command name, e.g.
// "!uptime" => Cmd{"uptime"}
// "!mark ohai" => Cmd{Token: "mark", Cmds: []Cmd{Cmd{"*"}}}
// "!prefs set" => Cmd{Token: "prefs", MustSubCmd: true, Cmds: []Cmd{Cmd{"set"}}}
type Cmd struct {
	Token      string   `json:"token"` // * => slurp everything remaining
	Usage      string   `json:"usage"`
	Params     []*Param `json:"parameters"`
	SubCmds    []*Cmd   `json:"subcommands"`
	Prev       *Cmd     // parent command, nil for root
	MustSubCmd bool     // a subcommand is always required
}

// does anyone else feel weird writing ParamInsts?
type CmdInst struct {
	Cmd        *Cmd         `json:"command"`
	SubCmdInst *CmdInst     `json:"subcommand"`
	ParamInsts []*ParamInst `json:"parameters"`
	Remainder  []string     `json:"remainder"` // args left over after parsing, usually empty
}

// Param defines a parameter of a command.
type Param struct {
	Key      string   `json:"key"`      // the "foo" in --foo, -f, foo=bar
	Position int      `json:"position"` // positional arg index
	Usage    string   `json:"usage"`    // usage string for generating help
	Required bool     `json:"required"` // whether or not this parameter is required
	Boolean  bool     `json:"boolean"`  // true for flags that default "true" with no arg
	ValidRE  string   `json:"validre"`  // a regular expression for validity checking
	Aliases  []string `json:"aliases"`  // parameter aliases, e.g. foo => f
	validre  *regexp.Regexp
	cmd      *Cmd
}

// ParamInst is an instance of a (parsed) parameter.
type ParamInst struct {
	Param *Param `json:"param"`
	Found bool   `json:"found"`   // was the parameter set?
	Value string `json:"value"`   // provided value or the default
	Cmd   *Cmd   `json:"command"` // the command the parameter is attached to
	Arg   string `json:"arg"`     // the original/unmodified argument (e.g. --foo, -f)
	Key   string `json:"key"`     // the key, e.g. "foo"
}

// RequiredParamNotFound is returned when a parameter has Required=true
// and a method was used to access the value but no value was set in the
// command.
type RequiredParamNotFound struct {
	Param *Param
}

// Error fulfills the Error interface.
func (e RequiredParamNotFound) Error() string {
	return fmt.Sprintf("Parameter %q is required but not set.", e.Param.Key)
}

// UnsupportedTimeFormatError is returned when a provided time string cannot
// be parsed with one of the pre-defined time formats.
type UnsupportedTimeFormatError struct {
	Value string
}

// Error fulfills the Error interface.
func (e UnsupportedTimeFormatError) Error() string {
	return fmt.Sprintf("Time string %q does not appear to be in a supported format.", e.Value)
}

// NewCmd returns an initialized *Cmd.
func NewCmd(token string) *Cmd {
	cmd := Cmd{
		Token:   token,
		Params:  make([]*Param, 0),
		SubCmds: make([]*Cmd, 0),
	}
	return &cmd
}

// subcmds makes sure the SubCmds list is initialized and returns the list.
func (c *Cmd) subcmds() []*Cmd {
	if c.SubCmds == nil {
		c.SubCmds = make([]*Cmd, 0)
	}

	return c.SubCmds
}

// params makes sure the Params list is initialized and returns the list.
func (c *Cmd) params() []*Param {
	if c.Params == nil {
		c.Params = make([]*Param, 0)
	}

	return c.Params
}

// AddParam creates and adds a parameter to the command handle and returns
// the new parameter.
func (c *Cmd) AddParam(key string, required bool) *Param {
	p := Param{
		Key:      key,
		Required: required,
		cmd:      c,
	}

	c.Params = append(c.params(), &p)

	return &p
}

// AddPParam adds a positional parameter to the command and returns the
// new parameter.
func (c *Cmd) AddPParam(position int, required bool) *Param {
	p := Param{
		Position: position,
		Required: required,
		cmd:      c,
	}

	c.Params = append(c.params(), &p)

	return &p
}

// AddAlias adds an alias to the parameter and returns the paramter.
func (p *Param) AddAlias(key, alias string) *Param {
	p.Aliases = append(p.aliases(), alias)
	return p
}

// AddUsage sets the usage string for the command. Returns the command.
func (c *Cmd) AddUsage(usage string) *Cmd {
	c.Usage = usage
	return c
}

// AddUsage sets the usage string for the paremeter. Returns the parameter.
func (p *Param) AddUsage(usage string) *Param {
	p.Usage = usage
	return p
}

// Cmd returns the command the parameter belongs to. Only really useful for
// chained methods since it will panic if the private command field isn't set.
func (p *Param) Cmd() *Cmd {
	if p.cmd == nil {
		panic("Can't call Cmd() on this Param because p.cmd is nil!")
	}

	return p.cmd
}

// AddCmd adds a subcommand to the handle and returns the new (sub-)command.
func (c *Cmd) AddCmd(token string) *Cmd {
	sub := Cmd{
		Token: token,
		Prev:  c,
	}

	c.SubCmds = append(c.subcmds(), &sub)

	return &sub
}

// GetParam gets a parameter by its key. Returns nil for no match.
// Only processes the handle, no recursion.
func (c *Cmd) GetParam(key string) *Param {
	for _, p := range c.params() {
		if p.Key == key {
			return p
		}
	}

	return nil
}

func (c *Cmd) HasParam(key string) bool {
	for _, p := range c.params() {
		if p.Key == key {
			return true
		}
	}

	return false
}

// FindParam recursively finds any parameter defined in the command or its
// subcommands and returns the param. nil on miss.
func (c *Cmd) FindParam(key string) (*Cmd, *Param) {
	p := c.GetParam(key)
	if p != nil {
		return c, p
	}

	for _, sc := range c.subcmds() {
		_, p = sc.FindParam(key)
		if p != nil {
			return sc, p
		}
	}

	return nil, nil
}

// GetSubCmd gets a subcommand by its token. Returns nil for no match.
func (c *Cmd) GetSubCmd(token string) *Cmd {
	for _, s := range c.subcmds() {
		if s.Token == token {
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
	topInst := CmdInst{
		Cmd:       c,
		Remainder: []string{},
	}

	// the current subcommand - changes during parsing
	curInst := &topInst

	if len(argv) == 1 {
		log.Panicf("TODO: handle commands with no arguments gracefully")
		return nil
	}

	var skipNext bool
	var looseParams []*ParamInst

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

		if strings.Contains(arg, "=") {
			// looks like a key=value or --key=value parameter
			// could be --foo=bar but all that matters is the "foo"
			// could be --foo=true for .Boolean=true and that's fine too
			kv := strings.SplitN(arg, "=", 2)
			key = strings.TrimLeft(kv[0], "-")
			value = kv[1]
		} else if looksLikeParam(arg) {
			// looks like a parameter
			// e.g. --foo bar -f bar
			key = strings.TrimLeft(arg, "-")
			if nextExists && !looksLikeParam(next) {
				value = next
				skipNext = true
			} else {
				log.Printf("next arg, %q, appears to be a parameter", next)
			}
		} else if curInst.Cmd.HasSubCmd(arg) {
			// arg seems to be a subcommand token
			// retreive the Cmd handle, create a command instance for the
			// subcommand, then install it in the return tree
			sub := curInst.Cmd.FindSubCmd(arg)
			inst := CmdInst{Cmd: sub, Remainder: []string{}}
			curInst.SubCmdInst = &inst

			// advance to the next subcommand
			curInst = &inst
			continue
		} else {
			// leftover args go in .Remainder
			curInst.Remainder = append(curInst.remainder(), arg)
			continue
		}

		inst := ParamInst{
			Key:   key,
			Arg:   arg,
			Found: true,
			Value: value,
		}

		// always prefer matching the current subcmd
		// TODO: check aliases, possibly in GetParam
		if p := curInst.Cmd.GetParam(key); p != nil {
			inst.Param = p
			inst.Cmd = p.Cmd()

			curInst.ParamInsts = append(curInst.paraminsts(), &inst)
		} else {
			// not a parameter for the current subcommand, probably out of order or invalid
			// the subcommand might not be known yet. defer matching until all args are parsed
			looseParams = append(looseParams, &inst)
		}
	}

	// process out-of-order (and invalid) parameters in a second pass
	for _, inst := range looseParams {
		var found bool

		// search from the top CmdInst -> down and take the first match
		for search := &topInst; search != nil; search = search.SubCmdInst {
			if p := search.Cmd.GetParam(inst.Key); p != nil {
				inst.Param = p
				inst.Cmd = p.Cmd()

				search.ParamInsts = append(search.paraminsts(), inst)

				found = true

				break
			}
		}

		// TODO: handle invalid parameters gracefully
		if !found {
			log.Panicf("Bug or invalid parameter %q", inst.Key)
		}
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

// FindSubCmd looks for a subcommand defined with the provided token.
func (c *Cmd) FindSubCmd(token string) *Cmd {
	for _, sc := range c.subcmds() {
		if sc.Token == token {
			return sc
		}
	}

	return nil
}

// Param gets a parameter instance by its key.

// HasSubCmd returns whether or not the proivded token is defined as a subcommand.
func (c *Cmd) HasSubCmd(token string) bool {
	sc := c.FindSubCmd(token)
	return sc != nil
}

// SubCmdToken returns the subcommand's token string. Returns empty string
// if there is no subcommand.
func (c *CmdInst) SubCmdToken() string {
	if c.SubCmdInst != nil {
		return c.SubCmdInst.Cmd.Token
	}

	return ""
}

// Param gets a parameter instance by its key.
func (c *CmdInst) GetParamInst(key string) *ParamInst {
	for _, p := range c.paraminsts() {
		if p.Param != nil && p.Param.Key == key {
			return p
		}
	}

	log.Panicf("%q.GetParamInst(%q) found not match", c.Cmd.Token, key)

	return nil
}

// paraminsts initializes the ParamInsts list on the fly and returns it.
// e.g. c.ParamInsts = append(c.paraminsts(), pi)
func (c *CmdInst) paraminsts() []*ParamInst {
	if c.ParamInsts == nil {
		c.ParamInsts = make([]*ParamInst, 0)
	}

	return c.ParamInsts
}

// paraminsts initializes the Remainder list on the fly and returns it.
// e.g. c.Remainder = append(c.remainder(), arg)
func (c *CmdInst) remainder() []string {
	if c.Remainder == nil {
		c.Remainder = make([]string, 0)
	}

	return c.Remainder
}

// Instance creates an instance of the parameter with the provided value and bound
// to the provided Cmd.
func (p *Param) Instance(value string, cmd *Cmd) *ParamInst {
	pi := ParamInst{
		Param: p,
		Found: true,
		Value: value,
		Cmd:   cmd,
	}
	return &pi
}

// aliases is an internal shortcut to initialize the aliases list on an
// as-needed basis.
func (p *Param) aliases() []string {
	if p.Aliases == nil {
		p.Aliases = make([]string, 0)
	}

	return p.Aliases
}

// String returns the value as a string. If the param is required and it was
// not set, RequiredParamNotFound is returned.
func (p *ParamInst) String() (string, error) {
	if !p.Found && p.Param.Required {
		return "", RequiredParamNotFound{p.Param}
	}

	return p.Value, nil
}

// String returns the value as an int. If the param is required and it was
// not set, RequiredParamNotFound is returned. Additionally, any errors in
// conversion are returned.
func (p *ParamInst) Int() (int, error) {
	if !p.Found {
		if p.Param.Required {
			return 0, RequiredParamNotFound{p.Param}
		} else {
			return 0, nil
		}
	}

	val, err := strconv.ParseInt(p.Value, 10, 64)
	return int(val), err // warning: doesn't handle overflow
}

// Float returns the value of the parameter as a float. If the value cannot
// be converted, an error will be returned. See: strconv.ParseFloat
func (p *ParamInst) Float() (float64, error) {
	if !p.Found {
		if p.Param.Required {
			return 0, RequiredParamNotFound{p.Param}
		} else {
			return 0, nil
		}
	}

	return strconv.ParseFloat(p.Value, 64)
}

// Bool returns the value of the parameter as a bool.
// If the value is required and not set, returns RequiredParamNotFound.
// If the value cannot be converted, an error will be returned.
// See: strconv.ParseBool
func (p *ParamInst) Bool() (bool, error) {
	if !p.Found {
		if p.Param.Required {
			return false, RequiredParamNotFound{p.Param}
		} else {
			return false, nil
		}
	}

	stripped := strings.Trim(p.Value, `'"`)
	return strconv.ParseBool(stripped)
}

// Duration returns the value of the parameter as a Go time.Duration.
// Day and Week (e.g. "1w", "1d") are converted to 168 and 24 hours respectively.
// If the value is required and not set, returns RequiredParamNotFound.
// If the value cannot be converted, an error will be returned.
// See: time.ParseDuration
func (p *ParamInst) Duration() (time.Duration, error) {
	duration := p.Value
	empty := time.Duration(0)

	if !p.Found {
		if p.Param.Required {
			return empty, RequiredParamNotFound{p.Param}
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

// Time returns the value of the parameter as a Go time.Time.
// Many formats are attempted before giving up.
// If the value is required and not set, returns RequiredParamNotFound.
// If the value cannot be converted, an error will be returned.
// See: TimeFormats in this package
// See: time.ParseDuration
func (p *ParamInst) Time() (time.Time, error) {
	if !p.Found {
		if p.Param.Required {
			return time.Time{}, RequiredParamNotFound{p.Param}
		} else {
			return time.Time{}, nil
		}
	}

	t := p.Value

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

// MustString returns the value as a string. If it was required/not-set,
// panic ensues. Empty string may be returned for not-required+not-set.
func (p *ParamInst) MustString() string {
	out, err := p.String()
	if p.Param.Required && err != nil {
		panic(err)
	}

	return out
}

// DefString returns the value as a string. Rules:
// If the param is required and it was not set, return the provided default.
// If the param is not required and it was not set, return the empty string.
// If the param is set and the value is "*", return the provided default.
// If the param is set, return the value.
func (p *ParamInst) DefString(def string) string {
	if !p.Found {
		if p.Param.Required {
			// not set, required
			return def
		} else {
			// not set, not required
			return ""
		}
	} else if p.Value == "*" {
		return def
	}

	out, err := p.String()
	if err != nil {
		return def
	}
	return out
}

// DefInt returns the value as an int. See DefString for the rules.
func (p *ParamInst) DefInt(def int) int {
	if !p.Found {
		if p.Param.Required {
			return def
		} else {
			return 0
		}
	} else if p.Value == "*" {
		return def
	}

	out, err := p.Int()
	if err != nil {
		return def
	}
	return out
}

// DefFloat returns the value as a float. See DefString for the rules.
func (p *ParamInst) DefFloat(def float64) float64 {
	if !p.Found {
		if p.Param.Required {
			return def
		} else {
			return 0
		}
	} else if p.Value == "*" {
		return def
	}

	out, err := p.Float()
	if err != nil {
		return def
	}
	return out
}

// DefBool returns the value as a bool. See DefString for the rules.
func (p *ParamInst) DefBool(def bool) bool {
	if !p.Found {
		if p.Param.Required {
			return def
		} else {
			return false
		}
	} else if p.Value == "*" {
		return def
	}

	out, err := p.Bool()
	if err != nil {
		return def
	}
	return out
}
