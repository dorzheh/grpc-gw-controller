// Author  <dorzheho@cisco.com>

package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	appcommon "cisco.com/son/apphcd/app/common"
	"cisco.com/son/apphcd/app/controller"
	"cisco.com/son/apphcd/app/grpc/apphcmanager/version"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
)

const versionDesc = `
Show AppHoster Controller version.

The output will look something like this:

Controller: &version.Version{Version:"v1.0.0",ApiVersion: "v1", GitCommit:"4f97233d2cc2c7017b07f94211e55bb2670f990d", GitTreeState:"clean"}
`

// Variables
var (
	configFile string // File containing
	levelDebug bool   // Whether need to set debug level for logs
	ver        bool   // Print version
)

// This represents the base command when called without any sub-commands
var RootCmd = &cobra.Command{
	Use:   "apphcd",
	Short: "Cisco SON AppHoster Controller",
	Long:  `To get started run apphcd`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return serve()
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	// Initialize configuration
	cobra.OnInitialize(initConfig)

	// Set debug level for all logs
	RootCmd.PersistentFlags().BoolVarP(&levelDebug, "debug", "d", false, "run controller in debug mode")

	// Prints the version
	RootCmd.PersistentFlags().BoolVarP(&ver, "version", "v", false, "output version")
}

func formatVersion(v *version.Version, short bool) string {
	if short {
		return fmt.Sprintf("%s+g%s", v.Version, v.GitCommit[:7])
	}
	return fmt.Sprintf("%#v", v)
}

// initConfig instantiate environment variables
func initConfig() {

	if ver {
		_, err := fmt.Fprintln(os.Stdout, formatVersion(version.New(), false))
		if err != nil {
			logrus.Fatal(err)
		}

		os.Exit(0)
	}

	if configFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(configFile)
	}

	viper.SetConfigName("apphc") // name of config file (without extension)
	viper.AddConfigPath("/opt/cisco/apphc/config")
	viper.AddConfigPath(appcommon.ApphcHomePath) // adding home directory as first search path

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Config file: ", viper.ConfigFileUsed())

	// Set prefix for environment variables
	// It will check for a environment variable with a name matching the key
	// uppercased and prefixed with the EnvPrefix if set.
	viper.SetEnvPrefix("apphc")

	// Bind keys to appropriate environment variables
	err := viper.BindEnv(appcommon.EnvApphcLogFormat,
		appcommon.EnvApphSvcsUrlExternalIp,
		appcommon.EnvApphExternalIp,
		appcommon.EnvApphcNetworkPort,
		appcommon.EnvApphcCachePath,
		appcommon.EnvApphcAppsUpgradePolicyRecreate,
		appcommon.EnvApphcAppsRollbackEnabled,
		appcommon.EnvApphcInternalAuthorizationEnabled,
		appcommon.EnvApphcBearerToken,
		appcommon.EnvApphcPrivateDockerRegistry,
		appcommon.EnvApphcAppFlexApiHost,
		appcommon.EnvApphcAppFlexApiPort,
		appcommon.EnvApphcAdaptersRancherEnabled,
		appcommon.EnvApphMasterNodeUser,
		appcommon.EnvApphMasterNodeIp,
		appcommon.EnvApphcPurgeAppMetadata,
		rancher.EnvApphcAdaptersRancherClusterName,
		rancher.EnvApphcAdaptersRancherServerEndpoint,
		rancher.EnvApphcAdaptersRancherServerCredsToken,
		rancher.EnvApphcAdaptersRancherCatalogProto,
		rancher.EnvApphcAdaptersRancherCatalogPassword,
		rancher.EnvApphcAdaptersRancherTemplatesCatalogBranch,
		rancher.EnvApphcAdaptersRancherAppsCatalogName,
		rancher.EnvApphcAdaptersRancherAppsCatalogBranch)

	if err != nil {
		logrus.Fatal(err)
	}

	// Set default values
	viper.SetDefault(appcommon.EnvApphcLogFormat, appcommon.LogFormatText)
	viper.SetDefault(appcommon.EnvApphcNetworkPort, 10000)
	viper.SetDefault(appcommon.EnvApphcCachePath, "/tmp/.cache")
	viper.SetDefault(appcommon.EnvApphcAppsUpgradePolicyRecreate, true)
	viper.SetDefault(appcommon.EnvApphcAppsRollbackEnabled, true)
	viper.SetDefault(appcommon.EnvApphcInternalAuthorizationEnabled, true)
	viper.SetDefault(appcommon.EnvApphcBearerToken, "YXBwaG9zdGVyLWRlcGxveWVyLTIwMTgK") // apphoster-deployer-2018 in base64
	viper.SetDefault(appcommon.EnvApphcAdaptersRancherEnabled, true)
	viper.SetDefault(appcommon.EnvApphMasterNodeUser, "intucell")
	viper.SetDefault(appcommon.EnvApphcAppFlexApiPort, 7000)
	viper.SetDefault(appcommon.EnvApphcPurgeAppMetadata, true)
	viper.SetDefault(rancher.EnvApphcAdaptersRancherClusterName, "apphoster")
	viper.SetDefault(rancher.EnvApphcAdaptersRancherCatalogProto, "http")
	viper.SetDefault(rancher.EnvApphcAdaptersRancherCatalogPassword, "catalog")
	viper.SetDefault(rancher.EnvApphcAdaptersRancherTemplatesCatalogBranch, "master")
	viper.SetDefault(rancher.EnvApphcAdaptersRancherAppsCatalogName, "son-flex-apps")
	viper.SetDefault(rancher.EnvApphcAdaptersRancherAppsCatalogBranch, "master")
	viper.SetDefault(rancher.EnvApphcAdaptersRancherServerCredsToken, "kubeconfig-user-vxg8h:rf94p78gx2mk9fbmchvq7r9xbmzphhz42pltpskj2q8rdsf626n2sf")
	viper.AutomaticEnv()

	var formatter logrus.Formatter
	if viper.Get(appcommon.EnvApphcLogFormat).(appcommon.LogFormat) == appcommon.LogFormatJson {
		formatter = &logrus.JSONFormatter{}
	} else if viper.Get(appcommon.EnvApphcLogFormat).(appcommon.LogFormat) == appcommon.LogFormatText {
		formatter = &logrus.TextFormatter{}
	}

	logrus.SetFormatter(formatter)

	// Set debug level
	if levelDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	verification()
}

func verification() {

	if viper.GetString(appcommon.EnvApphcAppFlexApiHost) == "" {
		logrus.WithFields(logrus.Fields{
			"property": "APPHC_FLEX_API_HOST",
		}).Fatalf("configuration problem")
	}

	if viper.GetBool(appcommon.EnvApphcAdaptersRancherEnabled) {
		if viper.GetString(rancher.EnvApphcAdaptersRancherServerEndpoint) == "" {
			logrus.WithFields(logrus.Fields{
				"property": "APPHC_ADAPTERS_RANCHER_SERVER_ENDPOINT",
			}).Fatalf("configuration problem")
		}

		if viper.GetString(rancher.EnvApphcAdaptersRancherServerCredsToken) == "" {
			logrus.WithFields(logrus.Fields{
				"property": "APPHC_ADAPTERS_RANCHER_SERVER_CREDS_TOKEN",
			}).Fatalf("configuration problem")
		}
	}
}

func serve() error {
	// Set Controller port
	addrPort := fmt.Sprintf(":%s", viper.GetString("network_port"))
	// Initialize listener.
	// Listen on all interfaces
	conn, err := net.Listen("tcp", addrPort)
	if err != nil {
		return err
	}

	// Defer connection termination
	defer conn.Close()

	// Instantiate the server
	s := controller.New(viper.GetInt("network_port"), conn)
	return s.Start()
}
