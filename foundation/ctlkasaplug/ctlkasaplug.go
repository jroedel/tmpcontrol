package ctlkasaplug

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type KasaController struct {
	kasaPath string
}

func New(kasaPath string) (*KasaController, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, kasaPath, "--version")
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ctlkasaplug: %w", err)
	}
	return &KasaController{
		kasaPath: kasaPath,
	}, nil
}

func (k *KasaController) ControlDevice(ctx context.Context, host string, action Control) error {
	if action.String() == "" {
		return fmt.Errorf("empty action")
	}

	cmd := exec.CommandContext(ctx, k.kasaPath, "--host", host, "--type", "plug", action.String())

	var b bytes.Buffer
	cmd.Stdout = &b

	if err := cmd.Run(); err != nil {
		return err
	}
	stdOutput := b.String()
	//fmt.Println(stdOutput)
	if strings.Contains(stdOutput, "error") {
		return fmt.Errorf("we detected mention of an error in the stdout of our kasa command call: %s", stdOutput)
	}
	return nil
}

type Control int

const (
	// ControlOn Turn something on
	ControlOn Control = iota + 1
	// ControlOff Turn something off
	ControlOff
)

func (c Control) String() string {
	switch c {
	case ControlOn:
		return "on"
	case ControlOff:
		return "off"
	default:
		return ""
	}
}
