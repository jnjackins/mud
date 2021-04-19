package mud

import (
	"fmt"

	"github.com/fatih/color"
)

type Color struct {
	*color.Color
}

func (c *Color) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	switch s {
	case "red":
		c.Color = color.New(color.FgHiRed, color.Bold)
	case "orange":
		c.Color = color.New(color.FgYellow, color.Bold)
	case "yellow":
		c.Color = color.New(color.FgHiYellow, color.Bold)
	case "green":
		c.Color = color.New(color.FgGreen, color.Bold)
	case "blue":
		c.Color = color.New(color.FgHiCyan, color.Bold)
	case "purple":
		c.Color = color.New(color.FgMagenta, color.Bold)
	case "magenta":
		c.Color = color.New(color.FgHiMagenta, color.Bold)
	case "white":
		c.Color = color.New(color.FgHiWhite, color.Bold)
	default:
		return fmt.Errorf("unknown color %v", s)
	}

	return nil
}
