package exec

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/plugins/outputs"
	"github.com/influxdata/telegraf/plugins/serializers"
	shellquote "github.com/kballard/go-shellquote"
)

var sampleConfig = `
  ## Command
  command = "/usr/bin/mycollector --foo=bar"

  ## Timeout for each command to complete.
  timeout = "5s"

  ## Data format to output.
  ## Each data format has it's own unique set of configuration options, read
  ## more about them here:
  ## https://github.com/influxdata/telegraf/blob/master/docs/DATA_FORMATS_OUTPUT.md
  data_format = "influx"
`

type Exec struct {
	Command string
	Timeout internal.Duration

	serializer serializers.Serializer

	runner  Runner
	errChan chan error
}

type Runner interface {
	Run(*Exec, string, bytes.Buffer) error
}

type CommandRunner struct{}

func (c CommandRunner) Run(e *Exec, command string, buffer bytes.Buffer) error {
	splitCmd, err := shellquote.Split(command)
	if err != nil || len(splitCmd) == 0 {
		return fmt.Errorf("exec: unable to parse command, %s", err)
	}

	cmd := exec.Command(splitCmd[0], splitCmd[1:]...)
	cmd.Stdin = &buffer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := internal.RunTimeout(cmd, e.Timeout.Duration); err != nil {
		s := stderr.String()
		if s != "" {
			log.Printf("D! Command error: %q\n", s)
		}

		return fmt.Errorf("exec: %s for command %q", err, command)
	}

	return nil
}

func (e *Exec) Description() string {
	return "Send Telegraf metrics to commands that can input from stdin"
}

func (e *Exec) SampleConfig() string {
	return sampleConfig
}

func (e *Exec) SetSerializer(serializer serializers.Serializer) {
	e.serializer = serializer
}

func (e *Exec) Connect() error {
	return nil
}

func (e *Exec) Close() error {
	return nil
}

func (e *Exec) ProcessCommand(command string, buffer bytes.Buffer) {
	if err := e.runner.Run(e, command, buffer); err != nil {
		e.errChan <- err
		return
	}
}

func (e *Exec) Write(metrics []telegraf.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	var buffer bytes.Buffer
	for _, metric := range metrics {
		value, err := e.serializer.Serialize(metric)
		if err != nil {
			return err
		}
		buffer.Write(value)
	}

	if err := e.runner.Run(e, e.Command, buffer); err != nil {
		return err
	}

	return nil
}

func init() {
	outputs.Add("exec", func() telegraf.Output {
		return &Exec{
			runner:  CommandRunner{},
			Timeout: internal.Duration{Duration: time.Second * 5},
		}
	})
}
