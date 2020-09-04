package vsphere

import (
	"context"
	"fmt"
	"github.com/sethvargo/go-retry"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest/toolbox"
	"log"
	"time"

	//"github.com/roshankarande/go-vsphere/vsphere/guest/toolbox"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/types"
)

//func InvokeCommands(ctx context.Context,auth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager, data chan string, commands []string) error {
//
//	pmgr, err := opsmgr.ProcessManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	fmgr, err := opsmgr.FileManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	tboxClient := &toolbox.Client{
//		ProcessManager: pmgr,
//		FileManager:    fmgr,
//		Authentication: auth,
//		GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
//	}
//
//	err = tboxClient.RunCommands(ctx,data,commands)
//
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

//func InvokeScript(ctx context.Context,auth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager,data chan string, script string) error {
//
//	pmgr, err := opsmgr.ProcessManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	fmgr, err := opsmgr.FileManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	tboxClient := &toolbox.Client{
//		ProcessManager: pmgr,
//		FileManager:    fmgr,
//		Authentication: auth,
//		GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
//	}
//
//	err = tboxClient.RunScript(ctx,data,script)
//
//	if err != nil {
//		return err
//	}
//
//	return nil
//}


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

func InvokeRunCmd(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, command string, o chan CmdOutput, options  ...map[string]interface{}) error {

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		return fmt.Errorf("[vm] %s does not exist in [vc]", vmName)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

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
		return err
	}

	fmgr, err := opsmgr.FileManager(ctx)

	if err != nil {
		return err
	}

	authmgr, err := opsmgr.AuthManager(ctx)
	if err != nil {
		return err
	}

	tboxClient := &ToolBoxClient{
		Client:  toolbox.Client{
			ProcessManager: pmgr,
			FileManager:    fmgr,
			Authentication: baseGuestAuth,
			GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
		},
		AuthMgr: authmgr,
		vm : vm,
	}

	b, err := retry.NewConstant(20*time.Second)
	if err != nil {
		return err
	}

	err = retry.Do(ctx, retry.WithMaxDuration( 400 * time.Second, b), func(ctx context.Context) error {
		running, err := vm.IsToolsRunning(ctx)

		if err != nil {
			return err
		}

		if !running{
			fmt.Println("tools not running")
			return retry.RetryableError(fmt.Errorf("tools not running"))
		}

		if running{
			fmt.Println("tools are running")
		}

		return nil
	})

	if err := TestCredentials(ctx, baseGuestAuth, opsmgr); err != nil {
		return fmt.Errorf("authentication details not correct")
	}

	if err != nil {
		return fmt.Errorf("error with querying vmware tools status")
	}

	if err := tboxClient.TestCredentials(ctx); err != nil {
		return fmt.Errorf("authentication details not correct %s",err)
	}

	err = tboxClient.RunCmd(ctx,command,o)

	if err != nil {
		return err
	}

	return nil
}

func InvokeRunCmdBasic(ctx context.Context, c *govmomi.Client, vmName, guestUser, guestPassword string, command string, options ...map[string]interface{}) (*CmdOutput, error) {

	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmName)

	if err != nil {
		log.Fatal(err)
	}

	opsmgr := guest.NewOperationsManager(c.Client, vm.Reference())

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
		return nil,err
	}

	fmgr, err := opsmgr.FileManager(ctx)

	if err != nil {
		return nil, err
	}

	authmgr, err := opsmgr.AuthManager(ctx)
	if err != nil {
		return nil, err
	}

	tboxClient := &ToolBoxClient{
		Client:  toolbox.Client{
			ProcessManager: pmgr,
			FileManager:    fmgr,
			Authentication: baseGuestAuth,
			GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
		},
		AuthMgr: authmgr,
	}

	b, err := retry.NewConstant(20*time.Second)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("error with querying vmware tools status")
	}

	if err := tboxClient.TestCredentials(ctx); err != nil {
		return nil, fmt.Errorf("authentication details not correct %s",err)
	}

	cmdOutput, err := tboxClient.RunCmdBasic(ctx,command)

	if err != nil {
		return nil, err
	}

	return cmdOutput, nil
}



//func Upload(ctx context.Context, auth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager, f io.Reader, suffix, dst string, isDir bool) error {
//
//	pmgr, err := opsmgr.ProcessManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	fmgr, err := opsmgr.FileManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	c := &toolbox.Client{
//		ProcessManager: pmgr,
//		FileManager:    fmgr,
//		Authentication: auth,
//		GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
//	}
//
//	vcFile, err := c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "", suffix, "")
//
//	if err != nil {
//		return err
//	}
//
//	defer c.FileManager.DeleteFile(ctx, c.Authentication, vcFile)
//
//	p := soap.DefaultUpload
//	err = c.Upload(ctx, f, vcFile, p, &types.GuestFileAttributes{},true)
//	if err != nil {
//		return err
//	}
//
//	if isDir {
//		cmd := fmt.Sprintf("tar -xzvf %s -C %s", vcFile, dst)
//		c.RunSimpleCommands(ctx, []string{fmt.Sprintf("mkdir %s -Force",dst),cmd})
//	} else{
//		err = c.FileManager.MoveFile(ctx, c.Authentication, vcFile, dst,true)
//		if err != nil {
//			return err
//		}
//	}
//
//	return nil
//
//}

//func InvokeRunCmd(ctx context.Context,auth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager, command string, o chan CmdOutput) error {
//
//	pmgr, err := opsmgr.ProcessManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	fmgr, err := opsmgr.FileManager(ctx)
//
//	if err != nil {
//		return err
//	}
//
//	//tboxClient := &toolbox.Client{
//	//	ProcessManager: pmgr,
//	//	FileManager:    fmgr,
//	//	Authentication: auth,
//	//	GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
//	//}
//
//	authmgr, err := opsmgr.AuthManager(ctx)
//	if err != nil {
//		return err
//	}
//
//
//	tboxClient := &ToolBoxClient{
//		Client:  toolbox.Client{
//			ProcessManager: pmgr,
//			FileManager:    fmgr,
//			Authentication: auth,
//			GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
//		},
//		AuthMgr: authmgr,
//	}
//	err = tboxClient.RunCmd(ctx,command,o)
//
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
