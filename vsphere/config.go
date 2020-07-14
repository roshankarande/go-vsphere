package vsphere

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
	"net/url"
)

const (
	envURL      = "GOVMOMI_URL"
	envUserName = "GOVMOMI_USERNAME"
	envPassword = "GOVMOMI_PASSWORD"
	envInsecure = "GOVMOMI_INSECURE"
)


func NewClient(ctx context.Context) (*govmomi.Client, error) {

	u, err := soap.ParseURL(viper.GetString("URL"))
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(viper.GetString("USERNAME"),viper.GetString("PASSWORD"))

	return govmomi.NewClient(ctx, u, viper.GetBool("INSECURE"))
}

func init() {

	viper.SetConfigType("yaml")
	viper.SetConfigFile("vsphere/spec.yml")


	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("error reading spec - ",err)
	}

	viper.SetEnvPrefix("GOVMOMI")
	viper.AutomaticEnv()
}

func NewGovmomiClient(ctx context.Context, vSphereHost, vSphereUsername, vSpherePassword string) (*govmomi.Client, error) {

	u, err := soap.ParseURL(vSphereHost)
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(vSphereUsername, vSpherePassword)

	return govmomi.NewClient(ctx, u, true)
}