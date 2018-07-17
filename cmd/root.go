// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/naveego/plugin-pub-test/internal"
	"github.com/spf13/viper"
	"encoding/json"
	"io/ioutil"
	"github.com/pkg/errors"
	"github.com/manifoldco/promptui"
)

var pluginPath string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "test-pub",
	Short: "Driver tool for publisher plugins.",
	Long:  `Allows interactive testing of publisher plugins.`,

	RunE: func(cmd *cobra.Command, args []string) error{


		viper.BindPFlags(cmd.Flags())


		plugin := viper.GetString("plugin")
		scriptPath := viper.GetString("script")
		var script *internal.Script

		if plugin != "" {
			script = &internal.Script{
				PluginPath:plugin,
			}
		} else if scriptPath != "" {

			b, err := ioutil.ReadFile(scriptPath)
			if err != nil {
				return err
			}

			script = new(internal.Script)
			err = json.Unmarshal(b, script)
			if err != nil {
				return err
			}
		} else {
			return errors.New("you must provide either --plugin or --script")
		}
		logrus.SetLevel(logrus.DebugLevel)

		err := script.Run()

		if err == promptui.ErrAbort || err == promptui.ErrInterrupt {
			fmt.Println("Interrupted.")
			return nil
		}

		return err
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("plugin", "p", "", "The publisher to test.")
	rootCmd.Flags().StringP("script", "s", "", "The script to run.")
}
