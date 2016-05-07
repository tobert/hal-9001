package hal

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/netflix/hal-9001/hal"
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
	Cmd        *Cmd              `json:"command"`
	SubCmdInst *CmdInst          `json:"subcommand"`
	ParamInsts []*ParamInst      `json:"parameters"`
	Remainder  []string          `json:"remainder"` // args left over after parsing, usually empty
	args       map[string]string // used by the parser
	parent     *CmdInst          // used by the parser
}

// Param defines a parameter of a command.
type Param struct {
	Key      string `json:"key"`
	Usage    string `json:"usage"`
	Required bool   `json:"required"`
	Boolean  bool   `json:"boolean"` // true for flags that default "true" with no arg
	ValidRE  string `json:"validation_re2"`
	validre  *regexp.Regexp
}

// ParamInst is an instance of a (parsed) parameter.
type ParamInst struct {
	Param *Param `json:"param"`
	Found bool   `json:"found"`   // was the parameter set?
	Value string `json:"value"`   // provided value or the default
	Cmd   *Cmd   `json:"command"` // the command the parameter is attached to
	Arg   string `json:"arg"`     // the original/unmodified argument (e.g. --foo, -f)
	key   string `json:"key"`     // the parsed key (e.g. --foo: key = foo)
}

type RequiredParamNotFound struct {
	Param *Param
}

func (e RequiredParamNotFound) Error() string {
	return fmt.Sprintf("Parameter %q is required but not set.", e.Param.Key)
}

type UnsupportedTimeFormatError struct {
	Value string
}

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

// chaining methods - see example() below
// TODO: these are stubs
func (c *Cmd) AddParam(key, def string, required bool) *Cmd           { return c }
func (c *Cmd) AddAlias(name, alias string) *Cmd                       { return c }
func (c *Cmd) AddUsage(name, usage string) *Cmd                       { return c }
func (c *Cmd) AddPParam(position int, def string, required bool) *Cmd { return c }
func (c *Cmd) AddCmd(name string) *Cmd                                { return c }

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

// KeyParam adds "key" parameter with validation and can be chained.
// TODO: if there are going to be a few of these, maybe they should be generated.
func (c *Cmd) KeyParam(required bool) *Cmd {
	p := Param{
		Key:      "key",
		ValidRE:  "^key$",
		Usage:    "--key/-k the key string",
		Required: required,
	}

	p.validre = regexp.MustCompile("^key$")

	c.Params = append(c.Params, &p)

	return c
}

// TODO: these are stubs
func (c *Cmd) AddUserParam(def string, required bool) *Cmd   { return c }
func (c *Cmd) AddRoomParam(def string, required bool) *Cmd   { return c }
func (c *Cmd) AddBrokerParam(def string, required bool) *Cmd { return c }
func (c *Cmd) AddPluginParam(def string, required bool) *Cmd { return c }
func (c *Cmd) AddIdParam(def string, required bool) *Cmd     { return c }

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

	top := CmdInst{Cmd: c, Remainder: []string{}} // the top-level command
	current := &top                               // the subcommand - replaced if a subcommand is discovered

	if len(argv) == 1 {
		log.Panicf("TODO: handle commands with no arguments gracefully")
		return nil
	}

	var skipNext bool

	for i, arg := range argv[1:] {
		if skipNext {
			skipNext = false
			continue
		}

		var key, value, next string

		if i+1 < len(argv) {
			next = argv[i+1]
		}

		if strings.Contains(arg, "=") {
			kv := strings.SplitN(arg, "=", 2)
			// could be --foo=bar but all that matters is the "foo"
			// could be --foo=true for .Boolean=true and that's fine too
			key = kv[0]
			value = kv[1]
		} else if strings.HasPrefix(arg, "-") {
			// e.g. --foo bar -f bar
			key = arg
			if !looksLikeParam(next) {
				value = next
				skipNext = true
			}
		} else if current.Cmd.HasSubCmd(arg) {
			// advance to the next subcommand
			sub := current.Cmd.FindSubCmd(arg)
			inst := CmdInst{Cmd: sub, Remainder: []string{}}
			current.SubCmdInst = &inst
			current = &inst
			continue
		}

		// remove leading dashes
		key = strings.TrimLeft(key, "-")

		inst := ParamInst{
			key:   key,
			Arg:   arg,
			Found: true,
			Value: value,
		}

		// always prefer matching the current subcmd
		// TODO: check aliases, possibly in GetParam
		// TODO: consider renaming GetParam or at least some kind of internal one that
		// makes reading the code less confusing
		if pi := current.GetParam(key); pi != nil {
			inst.Cmd = current.Cmd
			inst.Param = pi.Param
		}

		// TODO: search upwards towards top to see if the cmd/subcmd has the parameter
		// and set Cmd/Param as above

		// some param instances will have Cmd/Param unset which is cleared up below

		current.ParamInsts = append(current.paraminsts(), &inst)
	}

	// at this point, parameters for subcommands that came before their subcommand
	// are attached to the wrong cmdinst and will be moved to the first parent
	// that has a matching parameter definition
	// e.g. !prefs --room core set
	for current = &top; true; current = top.SubCmdInst {
		if current == nil {
			break
		}

		pis := current.paraminsts()
		for j, inst := range pis {
			// Cmd is already set, nothing to do here
			if inst.Cmd != nil {
				continue
			}

			// search from the first cmd downwards and assign to the first match
			for search := &top; true; search = search.SubCmdInst {
				if search == nil {
					break
				}

				if param := search.GetParam(inst.Param.Key); param != nil {
					// set the correct command & param pointers
					inst.Cmd = search.Cmd
					inst.Param = param.Param

					// add to the new list
					search.ParamInsts = append(search.paraminsts(), inst)

					// remove from the old list
					current.ParamInsts = append(pis[:j], pis[j+1:]...)
				}
			}
		}
	}

	return &top
}

func looksLikeBoolValue(val string) bool {
	lcval := strings.ToLower(val)

	if strings.Contains(lcval, "true") {
		return true
	}

	if strings.Contains(lcval, "false") {
		return true
	}

	// TODO: do we really want this?
	//if val == "1" || val == "0" {
	//	return true
	//}

	return false
}

func looksLikeParam(key string) bool {
	if strings.HasPrefix(key, "-") {
		return true
	} else if strings.Contains(key, "=") {
		return true
	} else {
		return false
	}
}

func (c *Cmd) FindSubCmd(token string) *Cmd {
	for _, sc := range c.subcmds() {
		if sc.Token == token {
			return sc
		}
	}

	return nil
}

func (c *Cmd) HasSubCmd(token string) bool {
	sc := c.FindSubCmd(token)
	return sc != nil
}

// SubCmdKey returns the subcommand's key string. Returns empty string
// if there is no subcommand.
func (c *CmdInst) SubCmdToken() string {
	if c.SubCmdInst != nil {
		return c.SubCmdInst.Cmd.Token
	}
	return ""
}

// Param gets a parameter instance by its key.
func (c *CmdInst) GetParam(key string) *ParamInst {
	for _, p := range c.paraminsts() {
		if p.Param.Key == key {
			return p
		}
	}

	return nil
}

func (c *CmdInst) paraminsts() []*ParamInst {
	if c.ParamInsts == nil {
		c.ParamInsts = make([]*ParamInst, 0)
	}

	return c.ParamInsts
}

func (p *Param) Instance(value string, cmd *Cmd) *ParamInst {
	pi := ParamInst{
		Param: p,
		Found: true,
		Value: value,
		Cmd:   cmd,
	}
	return &pi
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
// panic ensues. Empty string is returned for not-required/not-set.
func (p *ParamInst) MustString() string {
	if p.Param.Required {
		out, err := p.String()
		if err != nil {
			panic(err)
		}
		return out
	} else {
		return p.DefString("")
	}
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

func example(evt hal.Evt) {
	// example 1
	oc := Cmd{
		Token:      "oncall",
		MustSubCmd: true,
		Usage:      "search Pagerduty escalation policies for a string",
		SubCmds: []*Cmd{
			NewCmd("cache-status"),
			NewCmd("cache-interval").AddPParam(0, "1h", true),
			NewCmd("*"), // everything else is a search string
		},
	}

	oc.GetSubCmd("cache-status").Usage = "check the status of the background caching job"
	oc.GetSubCmd("cache-interval").Usage = "set the background caching job interval"
	oc.GetSubCmd("*").Usage = "create a mark in time with an (optional) text note"
	// hmm maybe we can abuse varargs a bit without ruining safety....
	// basically achieves a type-safe kwargs...
	// NewCmd("*", Usage{"create a mark in time with an (optional) text note"})

	oci := oc.Process(evt.BodyAsArgv())

	switch oci.SubCmdToken() {
	case "cache-status":
		cacheStatus(&evt)
	case "cache-interval":
		cacheInterval(&evt, oci)
	case "*":
		search(&evt, oci)
	}

	// example 2
	// Alias: requiring explicit aliases instead of guessing seems right
	pc := NewCmd("prefs")
	pc.AddCmd("set").
		AddParam("key", "", true).
		AddAlias("key", "k"). // vertically aligned for your viewing pleasure
		AddParam("value", "", true).
		AddAlias("value", "v").
		AddParam("room", "", false).
		AddAlias("room", "r").
		AddUsage("room", "Set the room ID").
		AddParam("user", "", false).
		AddAlias("user", "u").
		AddParam("broker", "", false).
		AddAlias("broker", "b")
	// ^ in an init func, stuff below in the callback

	cmd := pc.Process(evt.BodyAsArgv())
	pref := hal.Pref{
		Key:    cmd.GetParam("key").MustString(),
		Value:  cmd.GetParam("value").MustString(),
		Room:   cmd.GetParam("room").DefString(evt.RoomId),
		User:   cmd.GetParam("user").DefString(evt.UserId),
		Broker: cmd.GetParam("borker").DefString(evt.BrokerName()),
	}

	switch cmd.SubCmdToken() {
	case "set":
		pref.Set()
		evt.Reply("saved!")
	case "get":
		got := pref.Get()
		tbl := hal.Prefs{got}.Table()
		evt.ReplyTable(tbl[0], tbl[1:])
	case "find":
		prefs := pref.Find()
		tbl := prefs.Table()
		evt.ReplyTable(tbl[0], tbl[1:])
	}

}

// stubs for example
func cacheStatus(evt *hal.Evt)                 {}
func cacheInterval(evt *hal.Evt, oci *CmdInst) {}
func search(evt *hal.Evt, oci *CmdInst)        {}
func setPref(evt *hal.Evt, oci *CmdInst)       {}
