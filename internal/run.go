package internal

import (
	"encoding/json"
	"github.com/naveego/plugin-pub-test/internal/pub"
	"os"
	"github.com/hashicorp/go-plugin"
	"github.com/naveego/dataflow-contracts/plugins"
	"os/exec"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/davecgh/go-spew/spew"
	"time"
	"os/signal"
	"syscall"
	"io"
	"github.com/sirupsen/logrus"
	"context"
	"github.com/spf13/viper"
	"io/ioutil"
	"errors"
)

type Script struct {
	PluginPath string
	Connect *pub.ConnectRequest
	Discover *pub.DiscoverShapesRequest
	DiscoveredShapes []*pub.Shape
	Publish *pub.PublishRequest
	publisher pub.PublisherClient
	log *logrus.Entry
}

func (s *Script) Run() error {

	logrus.SetLevel(logrus.DebugLevel)
	log := logrus.NewEntry(logrus.StandardLogger())
	s.log = log

	pluginLog := log.WithField("src", "plugin")
	// We're a host. Start by launching the plugin process.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion: plugins.PublisherProtocolVersion,
			MagicCookieKey: plugins.PublisherMagicCookieKey,
			MagicCookieValue: plugins.PublisherMagicCookieValue,
		},
		Plugins: map[string]plugin.Plugin{
			"publisher": pub.NewClientPlugin(log.WithField("src", "plugin")),
		},
		Cmd:              exec.Command(s.PluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Managed:          true,
		Logger: pub.AdaptHCLog(pluginLog),
	})

	log = log.WithField("src", "host")

	defer client.Kill()

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}

	raw, err := rpcClient.Dispense("publisher")
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}

	publisher := raw.(pub.PublisherClient)
	s.publisher = publisher


	for {
		if err := ErrorFeedback(s.TryConnect()); err != nil {
			return err
		}

		if err := ErrorFeedback(s.DiscoverOrRefreshShapes()); err != nil {
			return err
		}

		if err := ErrorFeedback(s.DoPublish()); err != nil {
			return err
		}

		sel := promptui.Select{
			Label: "Next",
			Items: []string{
				"Republish",
				"Reset",
				"Save Script",
				"Quit",
			},
		}
		_, action, err := sel.Run()
		if err != nil {
			return err
		}

		switch action {
		case "Republish":
			continue
		case "Reset":
			s.Connect = nil
			s.DiscoveredShapes = nil
			s.Discover = nil
			s.Publish = nil
			continue
		case "Save Script":
			pathPrompt := promptui.Prompt{
				Label:"Save to file",
				Default:viper.GetString("file"),
			}
			path, err := pathPrompt.Run()
			if err != nil {
				return err
			}
			b, err := json.Marshal(s)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(path, b, 0666)
			log.WithField("path", path).Info("Saved file.")
			return nil
		case "Quit":
			return nil
		}
	}
}

func ErrorFeedback(err error) error {
	if err == nil {
		return nil
	}
	sel := &promptui.Select{
		Label:fmt.Sprintf("Error: %s", err),
		Items:[]string{"Retry", "Quit"},
	}
	_, action, err := sel.Run()
	if err != nil {
		return err
	}
	if action == "Quit" {
		return errors.New("quitting")
	}
	return nil
}

func (s *Script) TryConnect() error {
	settingsMap := make(map[string]interface{})
	settingsJSON := ""
	var err error

	validateSettings := func(input string) error {

		e := json.Unmarshal([]byte(input), &settingsMap)
		if e != nil {
			return e
		}
		return nil
	}
	for {

		if s.Connect == nil {
			prompt := promptui.Prompt{
				Label:     "Connect settings (as JSON)",
				Validate:  validateSettings,
				Default:   settingsJSON,
				AllowEdit: true,
			}

			settingsJSON, err = prompt.Run()
			if err == promptui.ErrInterrupt {
				return err
			}
			if err != nil {
				fmt.Printf("Invalid settings: %s", err)
				continue
			}

			s.Connect = &pub.ConnectRequest{
				SettingsJson:settingsJSON,
			}
		}

		_, err = s.publisher.Connect(context.Background(), s.Connect)

		if err != nil {
			if ErrorFeedback(err) != nil {
				return err
			}

			s.Connect = nil
			continue
		}

		fmt.Println("Connected!")
		break
	}
	return nil
}

func (s *Script) DiscoverOrRefreshShapes() error {
	var err error

	if s.Discover == nil {
		s.Discover = &pub.DiscoverShapesRequest{
			SampleSize: 5,
		}
		sel := promptui.Select{
			Label: "Mode",
			Items: []string{pub.DiscoverShapesRequest_ALL.String(), pub.DiscoverShapesRequest_REFRESH.String()},
		}
		m, _, _ := sel.Run()
		s.Discover.Mode = pub.DiscoverShapesRequest_Mode(m)
	} else {
		s.log.WithField("req", s.Discover).Info("Using discover shapes request from script.")
	}

	switch s.Discover.Mode {
	case pub.DiscoverShapesRequest_REFRESH:
		fmt.Println("Refresh not implemented.")
	case pub.DiscoverShapesRequest_ALL:
		var discoverShapesResp *pub.DiscoverShapesResponse
		discoverShapesResp, err = s.publisher.DiscoverShapes(context.Background(), s.Discover)
		if err != nil {
			fmt.Printf("error: %s\n", err)
			return err
		}
		s.DiscoveredShapes = discoverShapesResp.Shapes
		fmt.Println("Discovered shapes:")
		spew.Dump(s.DiscoveredShapes)
	}

	return nil
}

func (s *Script) DoPublish() error {
	var err error
	var names []string

	if s.Publish == nil {
		for _, shape := range s.DiscoveredShapes {
			names = append(names, fmt.Sprintf("%s - %s", shape.Name, shape.Description))
		}

		sel := promptui.Select{
			Label: "Choose the shape to publish",
			Items: names,
		}
		m, _, _ := sel.Run()

		s.Publish = &pub.PublishRequest{
			Shape:s.DiscoveredShapes[m],
		}
	}

	startedAt := time.Now()

	var stream pub.Publisher_PublishStreamClient
	stream, err = s.publisher.PublishStream(context.Background(), s.Publish)

	if err != nil {
		return err
	}

	fmt.Printf("Expected count: %v\n", s.Publish.Shape.Count)

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, os.Kill, syscall.SIGTERM)

	count := 0
	for {
		var record *pub.Record
		record, err = stream.Recv()
		if record == nil && err == io.EOF {
			fmt.Println("Publish completed.")
			break
		} else if err != nil {
			fmt.Println("Publish ended with error: ", err)
			break
		} else {
			count++
			fmt.Println(record.Action.String(), record.DataJson)
		}
	}
	elapsed := time.Since(startedAt)
	fmt.Printf("Publish ran for %s\n", elapsed)
	fmt.Printf("Published %d records\n", count)

	return nil
}

