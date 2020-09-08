package vsphere

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform/terraform"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/guest/toolbox"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	"log"
	"strings"
	"time"
)

type ToolBoxClient struct {
	toolbox.Client
	AuthMgr *guest.AuthManager
}

type CmdOutput struct {
	Stdout string
	Stderr string
}

type exitError struct {
	error
	exitCode int
}

func (c ToolBoxClient) RunCmd(ctx context.Context, command string, options map[string]interface{}) error {

	cmdOutput := new(CmdOutput)

	_, outputSpecPresent := options["output"]

	if !outputSpecPresent{
		return fmt.Errorf("options parameter should have an outputSpec")
	}

	var o terraform.UIOutput

	o, ok := options["output"].(terraform.UIOutput)

	if !ok{
		return fmt.Errorf(`not able to cast options["output"] terraform.UIOutput`)
	}

	stdOutPath, err := c.mktemp(ctx)

	if err != nil {
		return err
	}
	defer c.rm(ctx, stdOutPath)

	stderrPath, err := c.mktemp(ctx)

	if err != nil {
		return err
	}

	defer c.rm(ctx, stderrPath)

	var args []string
	if strings.Contains(command,"1>") || strings.Contains(command,"2>") || strings.Contains(command,">") {
		args = []string{}
	}
	args = []string{"1>", stdOutPath, "2>", stderrPath}

	var path string

	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		args = append([]string{"-Command", fmt.Sprintf(`%s`, command)}, args...)
	default:
		//path = "/bin/bash"
		//arg := "'" + strings.Join(append([]string{cmd.Path}, args...), " ") + "'"
		//args = []string{"-c", arg}
		fmt.Errorf("not a windows machine")
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		WorkingDirectory: "",
		EnvVariables:     nil,
	}

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return err
	}

	rc := 0
	var l = []int64{0, 0} // l[0] - stdoutput len... l[1] - Stderr len

	for {

		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})

		if err != nil {
			if strings.Contains(err.Error(), "agent could not be contacted") {
				fmt.Println(err.Error())
				return nil
			}
			return err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10) // see what fits best.... time.Sleep?

			var buf = new(strings.Builder)

			buf, n, err := c.downloadHelperWindows(ctx, stdOutPath)
			if err != nil {
				return err
			}
			cmdOutput.Stdout = buf.String()[l[0]:n]
			l[0] = n

			o.Output(cmdOutput.Stdout)

			buf, n, err = c.downloadHelperWindows(ctx, stderrPath)
			if err != nil {
				return err
			}

			cmdOutput.Stderr = buf.String()[l[1]:n]
			l[1] = n

			o.Output(cmdOutput.Stderr)
			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)

	buf, n, err := c.downloadHelperWindows(ctx, stdOutPath)
	if err != nil {
		return err
	}

	cmdOutput.Stdout = buf.String()[l[0]:n]
	o.Output(cmdOutput.Stdout)

	buf, n, err = c.downloadHelperWindows(ctx, stderrPath)
	if err != nil {
		return err
	}

	cmdOutput.Stderr = buf.String()[l[1]:n]
	o.Output(cmdOutput.Stderr)

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}

	return nil
}

func (c ToolBoxClient) RunScript(ctx context.Context, script string, options map[string]interface{}) error{

	cmdOutput := new(CmdOutput)

	_, outputSpecPresent := options["output"]

	if !outputSpecPresent{
		return fmt.Errorf("options parameter should have an outputSpec")
	}

	var o terraform.UIOutput

	o, ok := options["output"].(terraform.UIOutput)

	if !ok{
		return fmt.Errorf(`not able to cast options["output"] terraform.UIOutput`)
	}

	ExecFile, err := c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "govmomi-", ".ps1", "")
	if err != nil {
		return err
	}
	defer c.rm(ctx, ExecFile)

	readerExecFile := strings.NewReader(script)

	if err != nil {
		return err
	}
	p := soap.DefaultUpload

	err = c.Upload(ctx, readerExecFile, ExecFile, p, &types.GuestFileAttributes{}, true)

	if err != nil {
		fmt.Println(err)
		return err
	}

	stdOutPath, err := c.mktemp(ctx)

	if err != nil {
		return err
	}
	defer c.rm(ctx, stdOutPath)

	stderrPath, err := c.mktemp(ctx)

	if err != nil {
		return err
	}

	defer c.rm(ctx, stderrPath)

	args := []string{"1>", stdOutPath, "2>", stderrPath}

	var path string

	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		args = append([]string{ExecFile}, args...)
	default:
		//path = "/bin/bash"
		//arg := "'" + strings.Join(append([]string{cmd.Path}, args...), " ") + "'"
		//args = []string{"-c", arg}
		fmt.Errorf("not a windows machine")
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		WorkingDirectory: "",
		EnvVariables:     nil,
	}

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return err
	}

	rc := 0
	var l = []int64{0, 0} // l[0] - stdoutput len... l[1] - Stderr len

	for {

		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})

		if err != nil {
			if strings.Contains(err.Error(), "agent could not be contacted") {
				fmt.Println(err.Error())
				return nil
			}
			return err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10) // see what fits best.... time.Sleep?

			var buf = new(strings.Builder)

			buf, n, err := c.downloadHelperWindows(ctx, stdOutPath)
			if err != nil {
				return err
			}
			cmdOutput.Stdout = buf.String()[l[0]:n]
			l[0] = n
			
			o.Output(cmdOutput.Stdout)

			buf, n, err = c.downloadHelperWindows(ctx, stderrPath)
			if err != nil {
				return err
			}

			cmdOutput.Stderr = buf.String()[l[1]:n]
			l[1] = n

			o.Output(cmdOutput.Stderr)
			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)

	buf, n, err := c.downloadHelperWindows(ctx, stdOutPath)
	if err != nil {
		return err
	}

	cmdOutput.Stdout = buf.String()[l[0]:n]
	o.Output(cmdOutput.Stdout)

	buf, n, err = c.downloadHelperWindows(ctx, stderrPath)
	if err != nil {
		return err
	}

	cmdOutput.Stderr = buf.String()[l[1]:n]
	o.Output(cmdOutput.Stderr)

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}
	return nil
}

func (c ToolBoxClient) RunCmdSync(ctx context.Context, command string) (*CmdOutput, error) {

	stdOutPath, err := c.mktemp(ctx)

	if err != nil {
		return nil, err
	}
	defer c.rm(ctx, stdOutPath)

	stderrPath, err := c.mktemp(ctx)

	if err != nil {
		return nil, err
	}

	defer c.rm(ctx, stderrPath)

	args := []string{"1>", stdOutPath, "2>", stderrPath}

	var path string

	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		args = append([]string{"-Command", fmt.Sprintf(`"& { %s }"`, command)}, args...)
	default:
		//path = "/bin/bash"
		//arg := "'" + strings.Join(append([]string{cmd.Path}, args...), " ") + "'"
		//args = []string{"-c", arg}
		fmt.Errorf("not a windows machine")
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		WorkingDirectory: "",
		EnvVariables:     nil,
	}

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return nil, err
	}

	rc := 0

	cmdOutput := new(CmdOutput)

	for {

		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})
		if err != nil {
			if strings.Contains(err.Error(), "agent could not be contacted") {
				return nil, nil
			}
			return nil, err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10) // see what fits best.... time.Sleep
			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)

	buf, _, err = c.downloadHelperWindows(ctx, stdOutPath)
	if err != nil {
		return nil, err
	}

	cmdOutput.Stdout = buf.String()

	buf, _, err = c.downloadHelperWindows(ctx, stderrPath)
	if err != nil {
		return nil, err
	}

	cmdOutput.Stderr = buf.String()

	if rc != 0 {
		return nil, &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}

	return cmdOutput, nil
}

func (c *ToolBoxClient) TestCredentials(ctx context.Context) error {
	return c.AuthMgr.ValidateCredentials(ctx, c.Authentication)
}

// customized Function
func (c *ToolBoxClient) UploadFile(ctx context.Context, dst string, f io.Reader,suffix string, isDir bool) error {

	filepath, err := c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "", suffix, "")
	if err != nil {
		return err
	}

	defer c.FileManager.DeleteFile(ctx, c.Authentication, filepath)

	p := soap.DefaultUpload
	err = c.Upload(ctx, f, filepath, p, &types.GuestFileAttributes{}, true)
	if err != nil {
		return err
	}

	if isDir {

		if _, err := c.RunCmdSync(ctx, fmt.Sprintf(`mkdir "%s" -Force`, dst)); err != nil {
			return err
		}

		cmd := fmt.Sprintf("tar -xzvf %s -C %s", filepath, dst)

		if _, err := c.RunCmdSync(ctx, cmd); err != nil {
			return err
		}

	} else {
		err = c.FileManager.MoveFile(ctx, c.Authentication, filepath, dst, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ToolBoxClient) rm(ctx context.Context, path string) {
	err := c.FileManager.DeleteFile(ctx, c.Authentication, path)
	if err != nil {
		log.Printf("rm %q: %s", path, err)    // just comment this out
	}
}

func (c *ToolBoxClient) mktemp(ctx context.Context) (string, error) {
	return c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "govmomi-", "", "")
}

// customized Function
func adjustEncodingtoWindows(r io.Reader) (io.Reader, error) {
	win16be := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM)
	// Make a transformer that is like win16be, but abides by BOM:
	utf16bom := unicode.BOMOverride(win16be.NewDecoder())

	// Make a Reader that uses utf16bom:
	unicodeReader := transform.NewReader(r, utf16bom)

	return unicodeReader, nil
}

// customized Function
func (c *ToolBoxClient) downloadHelperWindows(ctx context.Context, path string) (*strings.Builder, int64, error) {
	temp := new(strings.Builder)

	f, _, err := c.Download(ctx, path)
	if err != nil {
		return nil, 0, err
	}

	z, err := adjustEncodingtoWindows(f)
	if err != nil {
		return nil, 0, err
	}

	n, err := io.Copy(temp, z)
	if err != nil {
		return nil, 0, err
	}

	return temp, n, nil
}

func NewToolBoxClient(ctx context.Context, opsmgr *guest.OperationsManager,guestUser, guestPassword string,family types.VirtualMachineGuestOsFamily) (*ToolBoxClient,error) {

	auth := types.NamePasswordAuthentication{
		GuestAuthentication: types.GuestAuthentication{
			InteractiveSession: false,
		},
		Username: guestUser,
		Password: guestPassword,
	}

	baseGuestAuth := types.BaseGuestAuthentication(&auth)

	pmgr, err := opsmgr.ProcessManager(ctx)

	if err != nil {
		return nil, err
	}

	fmgr, err := opsmgr.FileManager(ctx)

	if err != nil {
		return nil, err
	}

	authmgr, err := opsmgr.AuthManager(ctx)
	if err != nil {
		return nil, err
	}

	return &ToolBoxClient{
		Client:  toolbox.Client{
			ProcessManager: pmgr,
			FileManager:    fmgr,
			Authentication: baseGuestAuth,
			GuestFamily:    family,
		},
		AuthMgr: authmgr,
	},nil
}
