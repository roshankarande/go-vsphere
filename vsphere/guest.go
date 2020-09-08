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

	"github.com/hashicorp/terraform/terraform"
	//"github.com/roshankarande/go-vsphere/vsphere/guest/toolbox"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	DefaultDelay  = time.Duration(20)
	DefaultTimeout = time.Duration(400)
)

func InvokeCommands(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, commands []string,  options  map[string]interface{}) error{

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return fmt.Errorf("[vm] %s does not exist in [vc]", vmName)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	delay, ok := options["delay"].(time.Duration)

	if !ok {
		delay = DefaultDelay
	}

	timeout, ok := options["timeout"].(time.Duration)

	if !ok {
		timeout = DefaultTimeout
	}

	b, err := retry.NewConstant(delay*time.Second)
	if err != nil {
		return err
	}

	for _, command := range commands {
		//fmt.Printf("[cmd]%s\n", command)

		err = retry.Do(ctx, retry.WithMaxDuration( timeout * time.Second, b), func(ctx context.Context) error {
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

		if err := tboxClient.RunCmd(ctx, command, options); err != nil {
			return err
		}
	}

	return nil
}

func InvokeCommandsSync(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, commands []string, options map[string]interface{}) error {

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return err
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())


	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	delay, ok := options["delay"].(time.Duration)

	if !ok {
		delay = DefaultDelay
	}

	timeout, ok := options["timeout"].(time.Duration)

	if !ok {
		timeout = DefaultTimeout
	}

	_, oSpecPresent := options["output"]

	var o terraform.UIOutput

	if oSpecPresent{
		o, ok = options["output"].(terraform.UIOutput)

		if !ok{
			return fmt.Errorf("not able to assert terraform.UIOutput")
		}
	}

	b, err := retry.NewConstant(delay*time.Second)
	if err != nil {
		return err
	}

	for _, command := range commands {
		fmt.Printf("[cmd]%s\n", command)

		err = retry.Do(ctx, retry.WithMaxDuration( timeout *time.Second, b), func(ctx context.Context) error {
			running, err := vm.IsToolsRunning(ctx)

			if err != nil {
				return err
			}

			if !running{
				//fmt.Println("tools not running")
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

		if oSpecPresent {
			if strings.TrimSpace(cmdOutput.Stdout) != "" {
				o.Output(cmdOutput.Stdout)
			}

			if strings.TrimSpace(cmdOutput.Stderr) != "" {
				o.Output(cmdOutput.Stderr)
			}

		}

		//fmt.Println(cmdOutput.Stdout)
		//fmt.Println(cmdOutput.Stderr)

	}

	return nil
}

func InvokeScript(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, script string,  options  map[string]interface{}) error{

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return fmt.Errorf("[vm] %s does not exist in [vc]", vmName)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	delay, ok := options["delay"].(time.Duration)

	if !ok {
		delay = DefaultDelay
	}

	timeout, ok := options["timeout"].(time.Duration)

	if !ok {
		timeout = DefaultTimeout
	}

	b, err := retry.NewConstant(delay*time.Second)
	if err != nil {
		return err
	}

	fmt.Printf("[executing script]")

	err = retry.Do(ctx, retry.WithMaxDuration( timeout * time.Second, b), func(ctx context.Context) error {
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

	return tboxClient.RunScript(ctx,script,options)

}

func Upload(ctx context.Context,c *govmomi.Client, vmName, guestUser, guestPassword string, f io.Reader,suffix, dst string, isDir bool, options map[string]interface{}) error {

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return fmt.Errorf("[vm] %s does not exist in [vc]", vmName)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

	tboxClient,err  := NewToolBoxClient(ctx,opsmgr,guestUser,guestPassword,types.VirtualMachineGuestOsFamilyWindowsGuest)

	if err != nil {
		return err
	}

	delay, ok := options["delay"].(time.Duration)

	if !ok {
		delay = DefaultDelay
	}

	timeout, ok := options["timeout"].(time.Duration)

	if !ok {
		timeout = DefaultTimeout
	}

	b, err := retry.NewConstant(delay*time.Second)
	if err != nil {
		return err
	}

	//fmt.Println("[uploading]")

	err = retry.Do(ctx, retry.WithMaxDuration( timeout * time.Second, b), func(ctx context.Context) error {
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
