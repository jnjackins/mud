package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
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
	th.SetStyle("label.spell-damage", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorRed, Bold: tui.DecorationOn})
	th.SetStyle("label.spell-area", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorMagenta, Bold: tui.DecorationOn})
	th.SetStyle("label.spell-buff", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorBlue, Bold: tui.DecorationOn})
	th.SetStyle("label.spell-debuff", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorYellow, Bold: tui.DecorationOn})
	th.SetStyle("label.spell-healing", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorGreen, Bold: tui.DecorationOn})
	th.SetStyle("label.spell-utility", tui.Style{Bg: tui.ColorBlack, Fg: tui.ColorWhite, Bold: tui.DecorationOn})

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
		buffsUp := make(map[string]event)
		buffsDown := make(map[string]event)
		buffNamesUp := make([]string, 0)
		buffNamesDown := make([]string, 0)
		charges := make(map[string]int)
		maxCharges := make(map[string]int)
		spellNames := make([]string, 0)
		for e := range events {
			switch e.status {
			case mem:
				charges[e.name] = e.charges
				maxCharges[e.name] = e.charges
				spellNames = spellNames[0:0]
				for name := range charges {
					spellNames = append(spellNames, name)
				}
				sort.Slice(spellNames, func(i, j int) bool {
					if spellKinds[spellNames[i]] < spellKinds[spellNames[j]] {
						return true
					} else if spellKinds[spellNames[i]] == spellKinds[spellNames[j]] {
						return spellNames[i] < spellNames[j]
					}
					return false
				})
			case cast:
				charges[e.name]--
			case up, down:
				abilities[e.name] = e
				abilityNames = abilityNames[0:0]
				for name := range abilities {
					abilityNames = append(abilityNames, name)
				}
				sort.Strings(abilityNames)
			case on:
				delete(buffsDown, e.name)
				buffsUp[e.name] = e
				buffNamesUp = buffNamesUp[0:0]
				for name := range buffsUp {
					buffNamesUp = append(buffNamesUp, name)
				}
				sort.Strings(buffNamesUp)
			case off:
				delete(buffsUp, e.name)
				buffsDown[e.name] = e
				buffNamesDown = buffNamesDown[0:0]
				for name := range buffsDown {
					buffNamesDown = append(buffNamesDown, name)
				}
				sort.Strings(buffNamesDown)
			}
			ui.Update(func() {
				spellsTbl.RemoveRows()
				abilitiesTbl.RemoveRows()
				buffsTbl.RemoveRows()
				for _, name := range spellNames {
					if charges[name] > 0 {
						s := fmt.Sprintf("%7.7s %s", shorten(name), bar(charges[name], maxCharges[name]))
						l := tui.NewLabel(s)
						switch spellKinds[name] {
						case damage:
							l.SetStyleName("spell-damage")
						case area:
							l.SetStyleName("spell-area")
						case buff:
							l.SetStyleName("spell-buff")
						case debuff:
							l.SetStyleName("spell-debuff")
						case healing:
							l.SetStyleName("spell-healing")
						}
						spellsTbl.AppendRow(l)
					}
				}
				for _, name := range abilityNames {
					l := tui.NewLabel(name)
					switch abilities[name].status {
					case up:
						l.SetStyleName("status-up")
					case down:
						l.SetStyleName("status-down")
					}
					abilitiesTbl.AppendRow(l)
				}
				for _, name := range buffNamesDown {
					if _, ok := buffsDown[name]; !ok {
						continue
					}
					l := tui.NewLabel(name)
					l.SetStyleName("status-off")
					buffsTbl.AppendRow(l)
				}
				for _, name := range buffNamesUp {
					if _, ok := buffsUp[name]; !ok {
						continue
					}
					s := name
					dur := buffsUp[name].remaining
					if dur != 0 {
						s += fmt.Sprintf(" %v", dur.Round(time.Second))
					}
					l := tui.NewLabel(s)
					if dur > 10*time.Second {
						l.SetStyleName("status-warn")
					} else if dur > 0 && dur < 10*time.Second {
						l.SetStyleName("status-alarm")
					} else {
						l.SetStyleName("status-on")
					}
					buffsTbl.AppendRow(l)
				}
			})
		}
	}()

	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}
}

func shorten(name string) string {
	if fields := strings.Fields(name); len(fields) > 1 {
		for i := 0; i < len(fields)-1; i++ {
			fields[i] = fields[i][:1]
		}
		return strings.Join(fields, ".")
	}
	return name
}

func bar(charges, max int) string {
	s := []byte("         ")
	for i := len(s) - 1; i >= 0; i-- {
		if charges > 0 {
			s[i] = '#'
			charges--
			max--
		} else if max > 0 {
			s[i] = '-'
			max--
		} else {
			break
		}
	}
	return string(s)
}
