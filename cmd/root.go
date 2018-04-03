package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/samkreter/Kirix/kirix"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var sources string
var sourceConfig string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "kirix",
	Short: "kirix allows for serverless compute scaling.",
	Long:  `TODO`,
	Run: func(cmd *cobra.Command, args []string) {
		sepSources := strings.Split(sources, ",")

		f, err := kirix.New(sepSources, sourceConfig)
		if err != nil {
			log.Fatal(err)
		}
		f.Run()
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&sources, "sources", "", "Work sources comma seperated")
	RootCmd.PersistentFlags().StringVar(&sourceConfig, "source-config", "", "work source configuration file")
	RootCmd.PersistentFlags().StringVar(&kirixConfig, "kirix-config", "", "main configuration file")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if source == "" {
		fmt.Println("You must supply at least 1 work source using -sources")
		os.Exit(1)
	}

	if kirixConfig != "" {
		// Use config file from the flag.
		viper.SetConfigFile(kubeletConfig)
	} else {
		fmt.Println("Using default Kirix Configurations")
		//Figure out what that means.
	}

	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
