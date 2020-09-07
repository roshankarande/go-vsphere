package vsphere

import (
	"context"
	"fmt"
	"github.com/sethvargo/go-retry"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"io"
	"strings"
	"time"

	//"github.com/roshankarande/go-vsphere/vsphere/guest/toolbox"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/types"
)


func InvokeCommands(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, commands []string,  options  ...map[string]interface{}) error{

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return fmt.Errorf("[vm] %s does not exist in [vc]", vmName)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	b, err := retry.NewConstant(20*time.Second)
	if err != nil {
		return err
	}

	for _, command := range commands {
		fmt.Printf("[cmd]%s\n", command)

		err = retry.Do(ctx, retry.WithMaxDuration( 400 * time.Second, b), func(ctx context.Context) error {
			running, err := vm.IsToolsRunning(ctx)

			if err != nil {
				return err
			}

			if !running{
				//fmt.Println("tools not running")
				return retry.RetryableError(fmt.Errorf("tools not running"))
			}

			//if running{
			//	fmt.Println("tools are running")
			//}

			return nil
		})

		if err != nil {
			return fmt.Errorf("error with querying vmware tools status")
		}

		if err := tboxClient.TestCredentials(ctx); err != nil {
			return fmt.Errorf("authentication details not correct %s",err)
		}

		cmdOutput := make(chan CmdOutput) // you should create new channel for every command... else it would be a problem....
		e := make(chan error)
		go tboxClient.RunCmd(ctx,command,cmdOutput, e)

	loop:
		for {
			select {
			case output, ok := <-cmdOutput:
				if !ok {
					break loop
				}

				if strings.TrimSpace(output.Stdout) != "" {
					fmt.Println(output.Stdout)
				}

				if strings.TrimSpace(output.Stderr) != "" {
					fmt.Println(output.Stderr)
				}

			case err, ok := <-e:
				if !ok {
					break loop
				}
				fmt.Println(err)
			}
		}

	}

	return nil
}

func InvokeCommandsSync(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, commands []string, options ...map[string]interface{}) error {

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return err
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())


	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	b, err := retry.NewConstant(20*time.Second)
	if err != nil {
		return err
	}

	for _, command := range commands {
		fmt.Printf("[cmd]%s\n", command)

		err = retry.Do(ctx, retry.WithMaxDuration( 400*time.Second, b), func(ctx context.Context) error {
			running, err := vm.IsToolsRunning(ctx)

			if err != nil {
				return err
			}

			if !running{
				fmt.Println("tools not running")
				return retry.RetryableError(fmt.Errorf("tools not running"))
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("error with querying vmware tools status")
		}

		if err := tboxClient.TestCredentials(ctx); err != nil {
			return fmt.Errorf("authentication details not correct %s",err)
		}

		cmdOutput, err := tboxClient.RunCmdSync(ctx,command)

		if err != nil {
			return err
		}

		if strings.TrimSpace(cmdOutput.Stdout) != "" {
			fmt.Println(cmdOutput.Stdout)
		}

		if strings.TrimSpace(cmdOutput.Stderr) != "" {
			fmt.Println(cmdOutput.Stderr)
		}
	}

	return nil
}

func InvokeScript(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, script string,  options  ...map[string]interface{}) error{

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return fmt.Errorf("[vm] %s does not exist in [vc]", vmName)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	b, err := retry.NewConstant(20*time.Second)
	if err != nil {
		return err
	}

	fmt.Printf("[executing script]")

	err = retry.Do(ctx, retry.WithMaxDuration( 400 * time.Second, b), func(ctx context.Context) error {
			running, err := vm.IsToolsRunning(ctx)

			if err != nil {
				return err
			}

			if !running{
				//fmt.Println("tools not running")
				return retry.RetryableError(fmt.Errorf("tools not running"))
			}

			//if running{
			//	fmt.Println("tools are running")
			//}

			return nil
		})

	if err != nil {
		return fmt.Errorf("error with querying vmware tools status")
	}

	if err := tboxClient.TestCredentials(ctx); err != nil {
		return fmt.Errorf("authentication details not correct %s",err)
	}

	cmdOutput := make(chan CmdOutput) // you should create new channel for every command... else it would be a problem....
	e := make(chan error)
		
	go tboxClient.RunScript(ctx,script,cmdOutput, e)

	loop:
		for {
			select {
			case output, ok := <-cmdOutput:
				if !ok {
					break loop
				}

				if strings.TrimSpace(output.Stdout) != "" {
					fmt.Println(output.Stdout)
				}

				if strings.TrimSpace(output.Stderr) != "" {
					fmt.Println(output.Stderr)
				}

			case err, ok := <-e:
				if !ok {
					break loop
				}
				fmt.Println(err)
			}
		}

	return nil
}

func Upload(ctx context.Context,c *govmomi.Client, vmName, guestUser, guestPassword string,  f io.Reader,suffix, dst string, isDir bool) error {

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return fmt.Errorf("[vm] %s does not exist in [vc]", vmName)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	b, err := retry.NewConstant(20*time.Second)
	if err != nil {
		return err
	}

	fmt.Printf("[uploading]")

	err = retry.Do(ctx, retry.WithMaxDuration( 400 * time.Second, b), func(ctx context.Context) error {
		running, err := vm.IsToolsRunning(ctx)

		if err != nil {
			return err
		}

		if !running{
			//fmt.Println("tools not running")
			return retry.RetryableError(fmt.Errorf("tools not running"))
		}

		//if running{
		//	fmt.Println("tools are running")
		//}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error with querying vmware tools status")
	}

	if err := tboxClient.TestCredentials(ctx); err != nil {
		return fmt.Errorf("authentication details not correct %s",err)
	}

	return tboxClient.UploadFile(ctx,dst, f,suffix, isDir)
}

func TestCredentials(ctx context.Context, baseGuestAuth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager) error {

	authmgr, err := opsmgr.AuthManager(ctx)

	if err != nil {
		return err
	}

	err = authmgr.ValidateCredentials(ctx, baseGuestAuth)

	if err != nil {
		return err
	}

	return nil
}
