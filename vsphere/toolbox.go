package vsphere

import (
	"context"
	"fmt"
	"github.com/sethvargo/go-retry"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/guest/toolbox"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	"log"
	"os"
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

func (c ToolBoxClient) RunCmd(ctx context.Context, command string, o chan CmdOutput) error {
	defer close(o)

	output := []struct {
		io.Writer
		fd   string
		path string
	}{
		{os.Stdout, "1", ""},
		{os.Stderr, "2", ""},
	}

	var args []string

	for i, out := range output {
		if out.Writer == nil {
			continue
		}

		dst, err := c.mktemp(ctx)
		if err != nil {
			return err
		}

		defer c.rm(ctx, dst)

		args = append(args, out.fd+">", dst)
		output[i].path = dst
	}

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
		return err
	}

	rc := 0
	var l = []int64{0, 0} // l[0] - stdoutput len... l[1] - Stderr len

	temp := new(CmdOutput)

	for {

		b, err := retry.NewConstant(30 * time.Second)
		if err != nil {
			return err
		}

		var potentialRestart = false

		var procs []types.GuestProcessInfo

		err = retry.Do(ctx, retry.WithMaxRetries(4, b), func(ctx context.Context) error {
			if procs, err = c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid}); err != nil {
				if strings.Contains(err.Error(), "agent could not be contacted") {
					//fmt.Println("agent could not be contacted")
					potentialRestart = true
					return retry.RetryableError(err)

				}
				return err
			}
			return nil
		})

		if err != nil {
			return err
		}

		if potentialRestart {
			//fmt.Println("Potential restart")
			return nil
		}

		//procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})
		//
		//if err != nil {
		//	if strings.Contains(err.Error(),"agent could not be contacted"){
		//		fmt.Println(err.Error())
		//		return nil
		//	}
		//	return err
		//}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10) // see what fits best.... time.Sleep?

			for index, out := range output {
				var buf = new(strings.Builder)
				if out.Writer == nil {
					continue
				}

				f, n, err := c.Download(ctx, out.path)
				if err != nil {
					return err
				}

				z, err := adjustEncodingtoWindows(f)

				if err != nil {
					return err
				}

				n, _ = io.Copy(buf, z)

				switch index {
				case 0:
					temp.Stdout = buf.String()[l[index]:n]
				case 1:
					temp.Stderr = buf.String()[l[index]:n]
				default:
					return fmt.Errorf("index not 0 nor 1")
				}
				l[index] = n
			}

			o <- *temp
			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)
	for index, out := range output {
		if out.Writer == nil {
			continue
		}

		f, n, err := c.Download(ctx, out.path)
		if err != nil {
			return err
		}

		z, err := adjustEncodingtoWindows(f)

		if err != nil {
			return err
		}

		n, _ = io.Copy(buf, z)

		//fmt.Println(adjustEncodingtoWindows(f))

		switch index {
		case 0:
			temp.Stdout = buf.String()[l[index]:n]
		case 1:
			temp.Stderr = buf.String()[l[index]:n]
		default:
			return fmt.Errorf("index not 0 nor 1")
		}
	}

	o <- *temp
	//fmt.Println(len(temp.Stdout), len(temp.Stderr))

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}

	return nil
}

func (c ToolBoxClient) RunCmdBasic(ctx context.Context, command string) (*CmdOutput, error) {

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

	//b, err := retry.NewConstant(15 * time.Second)
	//if err != nil {
	//	return nil, err
	//}
	//var potentialRestart = false
	//
	//err = retry.Do(ctx, retry.WithMaxRetries(12, b), func(ctx context.Context) error {
	//	if err = c.TestCredentials(ctx); err != nil {
	//		if strings.Contains(err.Error(), "agent could not be contacted") {
	//			fmt.Println("agent could not be contacted")
	//			potentialRestart = true
	//			return retry.RetryableError(err)
	//
	//		}
	//		return err
	//	}
	//	return nil
	//})
	//
	//if err != nil {
	//	return nil, err
	//}
	//
	//if potentialRestart {
	//	//fmt.Println("Potential restart")
	//	return &CmdOutput{
	//		Stdout: "Potential Restart",
	//		Stderr: "",
	//	}, nil
	//}

	for {
		b, err := retry.NewConstant(30 * time.Second)
		if err != nil {
			return nil, err
		}

		var potentialRestart = false

		var procs []types.GuestProcessInfo

		err = retry.Do(ctx, retry.WithMaxRetries(4, b), func(ctx context.Context) error {
			if procs, err = c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid}); err != nil {
				if strings.Contains(err.Error(), "agent could not be contacted") {
					//fmt.Println("agent could not be contacted")
					potentialRestart = true
					return retry.RetryableError(err)

				}
				return err
			}
			return nil
		})

		if err != nil {
			return nil, err
		}

		if potentialRestart {
			//fmt.Println("Potential restart")
			return nil, nil
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

	if err != nil {
		return nil, err
	}

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

func (c *ToolBoxClient) rm(ctx context.Context, path string) {
	err := c.FileManager.DeleteFile(ctx, c.Authentication, path)
	if err != nil {
		log.Printf("rm %q: %s", path, err)
	}
}

func (c *ToolBoxClient) TestCredentials(ctx context.Context) error {
	if err := c.AuthMgr.ValidateCredentials(ctx, c.Authentication); err != nil {
		return err
	}

	return nil
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

//func (c *ToolBoxClient) downloadHelperWindows(ctx context.Context, path string) (io.Reader,error) {
//		f, _, err := c.Download(ctx, path)
//		if err != nil {
//			return nil, err
//		}
//
//		return adjustEncodingtoWindows(f)
//}

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
