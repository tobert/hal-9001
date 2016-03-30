// Package docker allows users to attach a Docker image to a room and interact
// with it over its stdin/stdout.
package docker

import (
	"os/exec"

	"github.com/netflix/hal-9001/hal"
)

const Name = "docker"

const Usage = `
Examples:
!docker images
!docker run
`

// Register makes this plugin available to the system.
func Register() {
	plugin := hal.Plugin{
		Name:  Name,
		Func:  docker,
		Regex: "^!docker",
	}

	plugin.Register()
}

func docker(evt hal.Evt) {
	argv := evt.BodyAsArgv()

	if len(argv) < 2 {
		evt.Reply(Usage)
		return
	}

	switch argv[1] {
	case "images":
		images(evt)
	case "run":
		if len(argv) < 3 {
			evt.Replyf("docker run requires an image id!\n%s", Usage)
			return
		}
		run(evt, argv)
	}
}

// TODO: the idea is to be able to run an interactive container that may be more
// than a single command, e.g. an old-school question/answer script that asks a
// few questions then does some work. This will probably require a timeout
// and some way to either signal which container you're messaging or spawn a
// DM room for the purpose and perhaps send the output back to the originating
// room. The DM approach is likely least complex, even across brokers.
func run(evt hal.Evt, argv []string) {
	// danger! insecure! Demo code ;)
	cmd := exec.Command("/usr/bin/docker", argv[1:]...)
	out, err := cmd.Output()
	if err != nil {
		evt.Replyf("Encountered an error while running 'docker run %s': %s", argv[2], err)
	}

	evt.Reply(string(out))
}

func images(evt hal.Evt) {
	cmd := exec.Command("/usr/bin/docker", "images")
	out, err := cmd.Output()
	if err != nil {
		evt.Replyf("Encountered an error while running 'docker images': %s", err)
	}

	evt.Reply(string(out))
}
