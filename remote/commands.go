package remote

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/viert/xc/log"
)

func (w *Worker) copy(task *Task) int {
	cmd := createSCPCmd(task.Hostname, task.LocalFilename, task.RemoteFilename, task.RecursiveCopy)
	return w._run(task, cmd)
}

func (w *Worker) runcmd(task *Task) int {
	cmd := createSSHCmd(task.Hostname, task.Cmd)
	return w._run(task, cmd)
}

func (w *Worker) tarcopy(task *Task) int {
	cmd := createTarCopyCmd(task.Hostname, task.LocalFilename, task.RemoteFilename)
	return w._run(task, cmd)
}

func createTarCopyCmd(host string, local string, remote string) *exec.Cmd {
	if remote == "" || remote == local {
		remote = "."
	}
	options := strings.Join(sshOpts(), " ")
	sshCmd := fmt.Sprintf("ssh -l %s %s %s", currentUser, options, host)
	tarCmd := fmt.Sprintf("tar c %s | %s tar x -C %s", local, sshCmd, remote)
	params := []string{"-c", tarCmd}
	log.Debugf("Created command bash %v", params)
	return exec.Command("bash", params...)
}

func createSCPCmd(host string, local string, remote string, recursive bool) *exec.Cmd {
	params := []string{}
	if recursive {
		params = []string{"-r"}
	}
	params = append(params, sshOpts()...)
	remoteExpr := fmt.Sprintf("%s@%s:%s", currentUser, host, remote)
	params = append(params, local, remoteExpr)
	log.Debugf("Created command scp %v", params)
	return exec.Command("scp", params...)
}

func createSSHCmd(host string, argv string) *exec.Cmd {
	params := []string{
		"-tt",
		"-l",
		currentUser,
	}
	params = append(params, sshOpts()...)
	params = append(params, host)
	params = append(params, getInterpreter()...)
	if argv != "" {
		params = append(params, "-c", argv)
	}
	log.Debugf("Created command ssh %v", params)
	return exec.Command("ssh", params...)
}

func getInterpreter() []string {
	switch currentRaise {
	case RTSudo:
		return strings.Split(sudoInterpreter, " ")
	case RTSu:
		return strings.Split(suInterpreter, " ")
	default:
		return strings.Split(noneInterpreter, " ")
	}
}
