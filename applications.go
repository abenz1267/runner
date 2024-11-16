package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/djherbis/times"
	"github.com/spf13/viper"
)

type entry struct {
	File         string
	Label        string
	Sub          string
	Icon         string
	Exec         string
	Terminal     bool
	Path         string
	Categories   []string
	InitialClass string
}

type Application struct {
	Generic entry
	Actions []entry
}

type Applications struct {
	entries []entry
}

func (a *Applications) Setup() {
	a.parse()
}

func (a *Applications) Query(term string) []Item {
	items := []Item{}

	for _, v := range a.entries {
		labels := []string{v.Label, v.Sub}
		labels = append(labels, v.Categories...)

		score := fuzzyScore(labels, term)

		item := Item{
			Labels:     map[string]string{"label": v.Label, "sub": v.Sub, "categories": strings.Join(v.Categories, ", ")},
			Icon:       v.Icon,
			Identifier: "",
			Provider:   "applications",
			score:      score,
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	return items
}

func (a *Applications) parse() {
	apps := []Application{}
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	dirs := xdg.ApplicationDirs
	flags := []string{"%f", "%F", "%u", "%U", "%d", "%D", "%n", "%N", "%i", "%c", "%k", "%v", "%m"}
	done := make(map[string]struct{})

	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			continue
		}

		filepath.WalkDir(d, func(path string, info fs.DirEntry, err error) error {
			if _, ok := done[info.Name()]; ok {
				return nil
			}

			if !info.IsDir() && filepath.Ext(path) == ".desktop" {
				file, err := os.Open(path)
				if err != nil {
					return err
				}

				defer file.Close()

				// matching := util.Fuzzy

				if viper.GetBool("applications.prioritizeNew") {
					if info, err := times.Stat(path); err == nil {
						target := time.Now().Add(-time.Minute * 5)

						mod := info.BirthTime()
						if mod.After(target) {
							// matching = util.AlwaysTopOnEmptySearch
						}
					}
				}

				scanner := bufio.NewScanner(file)

				app := Application{
					Generic: entry{
						// History:          true,
						// Matching:         matching,
						// RecalculateScore: true,
						File: path,
					},
					Actions: []entry{},
				}

				isAction := false
				skip := false

				for scanner.Scan() {
					line := scanner.Text()

					if strings.HasPrefix(line, "[Desktop Entry") {
						isAction = false
						skip = false
						continue
					}

					if skip {
						continue
					}

					if strings.HasPrefix(line, "[Desktop Action") {
						if !viper.GetBool("applications.actions") {
							skip = true
						}

						app.Actions = append(app.Actions, entry{})

						isAction = true
					}

					if strings.HasPrefix(line, "NoDisplay=") {
						nodisplay := strings.TrimPrefix(line, "NoDisplay=") == "true"

						if nodisplay {
							done[info.Name()] = struct{}{}
							return nil
						}

						continue
					}

					if strings.HasPrefix(line, "OnlyShowIn=") {
						onlyshowin := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "OnlyShowIn=")), ";")

						if slices.Contains(onlyshowin, desktop) {
							continue
						}

						done[info.Name()] = struct{}{}
						return nil
					}

					if strings.HasPrefix(line, "NotShowIn=") {
						notshowin := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "NotShowIn=")), ";")

						if slices.Contains(notshowin, desktop) {
							done[info.Name()] = struct{}{}
							return nil
						}

						continue
					}

					if !isAction {
						if strings.HasPrefix(line, "Name=") {
							app.Generic.Label = strings.TrimSpace(strings.TrimPrefix(line, "Name="))
							continue
						}

						if strings.HasPrefix(line, "Path=") {
							app.Generic.Path = strings.TrimSpace(strings.TrimPrefix(line, "Path="))
							continue
						}

						if strings.HasPrefix(line, "Categories=") {
							cats := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Categories=")), ";")
							app.Generic.Categories = append(app.Generic.Categories, cats...)
							continue
						}

						if strings.HasPrefix(line, "Keywords=") {
							cats := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Keywords=")), ";")
							app.Generic.Categories = append(app.Generic.Categories, cats...)
							continue
						}

						if strings.HasPrefix(line, "GenericName=") {
							app.Generic.Sub = strings.TrimSpace(strings.TrimPrefix(line, "GenericName="))
							continue
						}

						if strings.HasPrefix(line, "Terminal=") {
							app.Generic.Terminal = strings.TrimSpace(strings.TrimPrefix(line, "Terminal=")) == "true"
							continue
						}

						if strings.HasPrefix(line, "StartupWMClass=") {
							app.Generic.InitialClass = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "StartupWMClass=")))

							// if val, ok := openWindows[app.Generic.InitialClass]; ok {
							// 	app.Generic.OpenWindows = val
							// }

							continue
						}

						if strings.HasPrefix(line, "Icon=") {
							app.Generic.Icon = strings.TrimSpace(strings.TrimPrefix(line, "Icon="))
							continue
						}

						if strings.HasPrefix(line, "Exec=") {
							app.Generic.Exec = strings.TrimSpace(strings.TrimPrefix(line, "Exec="))

							for _, v := range flags {
								app.Generic.Exec = strings.ReplaceAll(app.Generic.Exec, v, "")
							}

							continue
						}
					} else {
						if strings.HasPrefix(line, "Exec=") {
							app.Actions[len(app.Actions)-1].Exec = strings.TrimSpace(strings.TrimPrefix(line, "Exec="))

							for _, v := range flags {
								app.Actions[len(app.Actions)-1].Exec = strings.ReplaceAll(app.Actions[len(app.Actions)-1].Exec, v, "")
							}
							continue
						}

						if strings.HasPrefix(line, "Name=") {
							app.Actions[len(app.Actions)-1].Label = strings.TrimSpace(strings.TrimPrefix(line, "Name="))
							continue
						}
					}
				}

				for k := range app.Actions {
					sub := app.Generic.Label

					if viper.GetBool("applications.showGeneric") && app.Generic.Sub != "" {
						sub = fmt.Sprintf("%s (%s)", app.Generic.Label, app.Generic.Sub)
					}

					app.Actions[k].Sub = sub
					app.Actions[k].Path = app.Generic.Path
					app.Actions[k].Icon = app.Generic.Icon
					app.Actions[k].Terminal = app.Generic.Terminal
					// app.Actions[k].Class = ApplicationsName
					// app.Actions[k].Matching = app.Generic.Matching
					app.Actions[k].Categories = app.Generic.Categories
					// app.Actions[k].History = app.Generic.History
					app.Actions[k].InitialClass = app.Generic.InitialClass
					// app.Actions[k].OpenWindows = app.Generic.OpenWindows
					// app.Actions[k].Prefer = true
					// app.Actions[k].RecalculateScore = true
					app.Actions[k].File = path
				}

				apps = append(apps, app)

				done[info.Name()] = struct{}{}
			}

			return nil
		})
	}

	for _, v := range apps {
		a.entries = append(a.entries, v.Generic)
		a.entries = append(a.entries, v.Actions...)
	}
}
