package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/marcusolsson/tui-go"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	events := watch(dir)

	th := tui.NewTheme()
	normal := tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorWhite}
	th.SetStyle("normal", normal)

	spellsTbl := tui.NewTable(1, 0)
	abilitiesTbl := tui.NewTable(1, 0)
	buffsTbl := tui.NewTable(1, 0)

	th.SetStyle("label.status-up", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorGreen, Bold: tui.DecorationOn})
	th.SetStyle("label.status-down", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorYellow, Bold: tui.DecorationOn})
	th.SetStyle("label.status-on", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorCyan, Bold: tui.DecorationOn})
	th.SetStyle("label.status-warn", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorMagenta, Bold: tui.DecorationOn})
	th.SetStyle("label.status-alarm", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorYellow, Bold: tui.DecorationOn})
	th.SetStyle("label.status-off", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorRed, Bold: tui.DecorationOn})

	left := tui.NewVBox(spellsTbl, tui.NewSpacer())
	left.SetSizePolicy(tui.Expanding, tui.Expanding)
	left.SetBorder(true)
	left.SetTitle("Spells")
	middle := tui.NewVBox(abilitiesTbl, tui.NewSpacer())
	middle.SetSizePolicy(tui.Expanding, tui.Expanding)
	middle.SetBorder(true)
	middle.SetTitle("Abilities")
	right := tui.NewVBox(buffsTbl, tui.NewSpacer())
	right.SetSizePolicy(tui.Expanding, tui.Expanding)
	right.SetBorder(true)
	right.SetTitle("Buffs")
	root := tui.NewHBox(left, middle, right)
	root.SetSizePolicy(tui.Expanding, tui.Expanding)

	ui, err := tui.New(root)
	if err != nil {
		log.Fatal(err)
	}

	ui.SetTheme(th)
	ui.SetKeybinding("Esc", func() { ui.Quit() })

	go func() {
		// todo: more generic widgets using interfaces
		abilities := make(map[string]event)
		abilityNames := make([]string, 0)
		buffs := make(map[string]event)
		buffNames := make([]string, 0)
		spells := make(map[string]int)
		spellNames := make([]string, 0)
		for e := range events {
			switch e.status {
			case mem:
				spells[e.name] = e.charges
				spellNames = spellNames[0:0]
				for name := range spells {
					spellNames = append(spellNames, name)
				}
				sort.Strings(spellNames)
			case cast:
				spells[e.name]--
			case up, down:
				abilities[e.name] = e
				abilityNames = abilityNames[0:0]
				for name := range abilities {
					abilityNames = append(abilityNames, name)
				}
				sort.Strings(abilityNames)
			case on, off:
				buffs[e.name] = e
				buffNames = buffNames[0:0]
				for name := range buffs {
					buffNames = append(buffNames, name)
				}
				sort.Strings(buffNames)
			}
			ui.Update(func() {
				spellsTbl.RemoveRows()
				abilitiesTbl.RemoveRows()
				buffsTbl.RemoveRows()
				for _, name := range spellNames {
					charges := spells[name]
					if charges > 0 {
						s := fmt.Sprintf("[%2d] %s", charges, name)
						spellsTbl.AppendRow(tui.NewLabel(s))
					}
				}
				for _, name := range abilityNames {
					l := tui.NewLabel(name)
					switch abilities[name].status {
					case up:
						l.SetStyleName("status-up")
						abilitiesTbl.AppendRow(l)
					case down:
						l.SetStyleName("status-down")
						abilitiesTbl.AppendRow(l)
					}
				}
				for _, name := range buffNames {
					s := name
					dur := buffs[name].remaining
					if dur != 0 {
						s += fmt.Sprintf(" %v", dur.Round(time.Second))
					}
					l := tui.NewLabel(s)
					switch buffs[name].status {
					case on:
						if dur > 10*time.Second {
							l.SetStyleName("status-warn")
						} else if dur > 0 && dur < 10*time.Second {
							l.SetStyleName("status-alarm")
						} else {
							l.SetStyleName("status-on")
						}
						buffsTbl.AppendRow(l)
					case off:
						l.SetStyleName("status-off")
						buffsTbl.AppendRow(l)
					}
				}
			})
		}
	}()

	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}
}
