package taskboard

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	native "goforge.dev/quicken/native"
)

var (
	desktopInk       = color.NRGBA{R: 21, G: 31, B: 43, A: 255}
	desktopRail      = color.NRGBA{R: 28, G: 42, B: 55, A: 255}
	desktopPaper     = color.NRGBA{R: 242, G: 238, B: 226, A: 255}
	desktopCard      = color.NRGBA{R: 255, G: 252, B: 242, A: 255}
	desktopText      = color.NRGBA{R: 35, G: 43, B: 50, A: 255}
	desktopMuted     = color.NRGBA{R: 104, G: 114, B: 119, A: 255}
	desktopRailText  = color.NRGBA{R: 231, G: 235, B: 227, A: 255}
	desktopTeal      = color.NRGBA{R: 29, G: 126, B: 120, A: 255}
	desktopAmber     = color.NRGBA{R: 219, G: 153, B: 54, A: 255}
	desktopCoral     = color.NRGBA{R: 202, G: 75, B: 66, A: 255}
	desktopLine      = color.NRGBA{R: 211, G: 205, B: 190, A: 255}
)

// DesktopView is a platform-specific workspace over the shared ForgeFlow
// model and message algebra.
func DesktopView() native.View[Model, Msg] {
	incidentList := &layout.List{Axis: layout.Vertical}
	detailList := &layout.List{Axis: layout.Vertical}
	return func(gtx layout.Context, model Model, ui *native.UI[Msg]) layout.Dimensions {
		ui.Theme.Bg = desktopPaper
		ui.Theme.Fg = desktopText
		ui.Theme.ContrastBg = desktopTeal
		ui.Theme.ContrastFg = desktopCard
		ui.Theme.TextSize = unit.Sp(15)
		fillDesktop(gtx, desktopInk, 0)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return desktopHeader(gtx, model, ui)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return desktopWorkspace(gtx, model, ui, incidentList, detailList)
			}),
		)
	}
}

func desktopHeader(gtx layout.Context, model Model, ui *native.UI[Msg]) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(70))
	return layout.Inset{Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H5(ui.Theme, "FORGEFLOW")
				title.Color = desktopRailText
				title.TextSize = unit.Sp(20)
				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(ui.Theme, "INCIDENT OPERATIONS")
				label.Color = color.NRGBA{R: 142, G: 167, B: 165, A: 255}
				return label.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(ui.Theme, syncLabel(model))
				if !model.Online || model.Error != "" { label.Color = desktopAmber } else { label.Color = desktopRailText }
				return label.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return desktopAction(gtx, ui, "desktop-sync", "SYNC", SyncRequested(), desktopTeal)
			}),
		)
	})
}

func desktopWorkspace(
	gtx layout.Context,
	model Model,
	ui *native.UI[Msg],
	incidentList, detailList *layout.List,
) layout.Dimensions {
	railWidth := gtx.Dp(unit.Dp(284))
	if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(850)) {
		railWidth = gtx.Dp(unit.Dp(224))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = railWidth, railWidth
			return desktopIncidentRail(gtx, model, ui, incidentList)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			fillDesktop(gtx, desktopPaper, 0)
			return layout.Inset{Top: unit.Dp(20), Right: unit.Dp(22), Bottom: unit.Dp(20), Left: unit.Dp(22)}.
				Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return desktopDetail(gtx, model, ui, detailList)
				})
		}),
	)
}

func desktopIncidentRail(
	gtx layout.Context,
	model Model,
	ui *native.UI[Msg],
	list *layout.List,
) layout.Dimensions {
	fillDesktop(gtx, desktopRail, 0)
	visible := VisibleIncidents(model)
	return layout.Inset{Top: unit.Dp(18), Right: unit.Dp(14), Bottom: unit.Dp(16), Left: unit.Dp(14)}.
		Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return desktopRailLabel(gtx, ui, fmt.Sprintf("INCIDENTS  %02d", len(visible)))
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return desktopEditor(gtx, ui, "desktop-search", model.Query, "Search title, owner, location", func(value string) Msg {
						return SearchChanged(value)
					}, desktopCard)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return desktopAction(gtx, ui, "desktop-all", "ALL", FilterRequested(FilterAll()), desktopTeal)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return desktopAction(gtx, ui, "desktop-critical", "CRITICAL", FilterRequested(FilterCritical()), desktopCoral)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return list.Layout(gtx, len(visible), func(gtx layout.Context, index int) layout.Dimensions {
						return desktopIncidentItem(gtx, ui, visible[index], model.SelectedID == visible[index].ID)
					})
				}),
			)
		})
}

func desktopIncidentItem(gtx layout.Context, ui *native.UI[Msg], incident Incident, selected bool) layout.Dimensions {
	background := desktopRail
	if selected { background = color.NRGBA{R: 42, G: 63, B: 75, A: 255} }
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return desktopSurface(gtx, background, 8, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return desktopTextLabel(gtx, ui, stringsUpper(string(incident.Severity))+"  ·  "+stringsUpper(string(incident.Status)), severityColor(incident.Severity), unit.Sp(11))
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return desktopAction(gtx, ui, fmt.Sprintf("desktop-incident-%d", incident.ID), incident.Title, IncidentSelected(incident.ID), background)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return desktopTextLabel(gtx, ui, incident.Location+"  ·  "+incident.Owner, desktopMuted, unit.Sp(12))
				}),
			)
		})
	})
}

func desktopDetail(gtx layout.Context, model Model, ui *native.UI[Msg], list *layout.List) layout.Dimensions {
	widgets := make([]layout.Widget, 0, 20)
	if incident, ok := selectedIncident(model); ok {
		widgets = append(widgets,
			func(gtx layout.Context) layout.Dimensions { return desktopIncidentHero(gtx, model, ui, incident) },
			func(gtx layout.Context) layout.Dimensions { return desktopSectionTitle(gtx, ui, "RESPONSE TASKS", fmt.Sprintf("%d", len(incident.Tasks))) },
		)
		for _, task := range incident.Tasks {
			current := task
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				label := "○  "+current.Title
				tone := desktopTeal
				if current.Done { label = "✓  "+current.Title; tone = desktopMuted }
				return desktopAction(gtx, ui, fmt.Sprintf("desktop-task-%d", current.ID), label, TaskToggled(current.ID), tone)
			})
		}
		widgets = append(widgets,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return desktopEditor(gtx, ui, "desktop-task-draft", model.TaskDraft, "Add response task", func(value string) Msg {
							return TaskDraftChanged(value)
						}, desktopCard)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return desktopAction(gtx, ui, "desktop-add-task", "ADD TASK", TaskAdded(), desktopTeal)
					}),
				)
			},
			func(gtx layout.Context) layout.Dimensions { return desktopSectionTitle(gtx, ui, "OPERATIONAL TIMELINE", fmt.Sprintf("%d", len(incident.Timeline))) },
		)
		for _, event := range incident.Timeline {
			current := event
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return desktopTimelineItem(gtx, ui, current)
			})
		}
		widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return desktopEditor(gtx, ui, "desktop-note-draft", model.NoteDraft, "Record an operational note", func(value string) Msg {
						return NoteDraftChanged(value)
					}, desktopCard)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return desktopAction(gtx, ui, "desktop-add-note", "ADD NOTE", NoteAdded(), desktopInk)
				}),
			)
		})
	}
	widgets = append(widgets,
		func(gtx layout.Context) layout.Dimensions { return desktopSectionTitle(gtx, ui, "OPEN NEW INCIDENT", "") },
		func(gtx layout.Context) layout.Dimensions { return desktopIncidentComposer(gtx, model, ui) },
	)
	if model.Error != "" {
		widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
			return desktopSurface(gtx, color.NRGBA{R: 255, G: 226, B: 216, A: 255}, 8, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
				return desktopTextLabel(gtx, ui, model.Error, desktopCoral, unit.Sp(13))
			})
		})
	}
	return list.Layout(gtx, len(widgets), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, widgets[index])
	})
}

func desktopIncidentHero(gtx layout.Context, model Model, ui *native.UI[Msg], incident Incident) layout.Dimensions {
	return desktopSurface(gtx, desktopCard, 12, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						title := material.H5(ui.Theme, incident.Title)
						title.Color = desktopText
						title.TextSize = unit.Sp(25)
						return title.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return desktopTextLabel(gtx, ui, stringsUpper(string(incident.Severity)), severityColor(incident.Severity), unit.Sp(12))
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return desktopTextLabel(gtx, ui, incident.Summary, desktopMuted, unit.Sp(14))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return desktopTextLabel(gtx, ui, incident.Location+"  ·  owner "+incident.Owner+"  ·  "+string(incident.Status), desktopText, unit.Sp(13))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopAction(gtx, ui, "desktop-severity-low", "LOW", SeverityChanged(incident.ID, SeverityLow), desktopMuted) }),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopAction(gtx, ui, "desktop-severity-high", "HIGH", SeverityChanged(incident.ID, SeverityHigh), desktopAmber) }),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopAction(gtx, ui, "desktop-severity-critical", "CRITICAL", SeverityChanged(incident.ID, SeverityCritical), desktopCoral) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopAction(gtx, ui, "desktop-advance", "ADVANCE STATUS →", StatusAdvanced(incident.ID), desktopTeal) }),
				)
			}),
		)
	})
}

func desktopIncidentComposer(gtx layout.Context, model Model, ui *native.UI[Msg]) layout.Dimensions {
	return desktopSurface(gtx, desktopCard, 12, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopEditor(gtx, ui, "desktop-new-title", model.IncidentDraft, "Incident title", func(value string) Msg { return IncidentDraftChanged(value) }, desktopPaper) }),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopEditor(gtx, ui, "desktop-new-summary", model.SummaryDraft, "Situation summary", func(value string) Msg { return SummaryDraftChanged(value) }, desktopPaper) }),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return desktopEditor(gtx, ui, "desktop-new-location", model.LocationDraft, "Location", func(value string) Msg { return LocationDraftChanged(value) }, desktopPaper) }),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return desktopEditor(gtx, ui, "desktop-new-owner", model.OwnerDraft, "Owner", func(value string) Msg { return OwnerDraftChanged(value) }, desktopPaper) }),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopAction(gtx, ui, "desktop-open", "OPEN INCIDENT", IncidentSubmitted(), desktopCoral) }),
				)
			}),
		)
	})
}

func desktopTimelineItem(gtx layout.Context, ui *native.UI[Msg], event Activity) layout.Dimensions {
	return desktopSurface(gtx, desktopCard, 7, unit.Dp(11), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopTextLabel(gtx, ui, event.At, desktopTeal, unit.Sp(12)) }),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopTextLabel(gtx, ui, stringsUpper(event.Kind), desktopMuted, unit.Sp(11)) }),
			layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return desktopTextLabel(gtx, ui, event.Body, desktopText, unit.Sp(13)) }),
		)
	})
}

func desktopSectionTitle(gtx layout.Context, ui *native.UI[Msg], title, count string) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopTextLabel(gtx, ui, title, desktopText, unit.Sp(13)) }),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return desktopTextLabel(gtx, ui, count, desktopMuted, unit.Sp(12)) }),
	)
}

func desktopRailLabel(gtx layout.Context, ui *native.UI[Msg], value string) layout.Dimensions {
	return desktopTextLabel(gtx, ui, value, desktopRailText, unit.Sp(12))
}

func desktopTextLabel(gtx layout.Context, ui *native.UI[Msg], value string, tone color.NRGBA, size unit.Sp) layout.Dimensions {
	label := material.Body1(ui.Theme, value)
	label.Color = tone
	label.TextSize = size
	return label.Layout(gtx)
}

func desktopEditor(
	gtx layout.Context,
	ui *native.UI[Msg],
	key, value, hint string,
	changed func(string) Msg,
	background color.NRGBA,
) layout.Dimensions {
	return desktopSurface(gtx, background, 7, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		return ui.Editor(gtx, key, value, hint, true, changed)
	})
}

func desktopAction(
	gtx layout.Context,
	ui *native.UI[Msg],
	key, label string,
	message Msg,
	tone color.NRGBA,
) layout.Dimensions {
	oldBackground, oldForeground := ui.Theme.ContrastBg, ui.Theme.ContrastFg
	ui.Theme.ContrastBg, ui.Theme.ContrastFg = tone, desktopCard
	dimensions := ui.Button(gtx, key, label, message)
	ui.Theme.ContrastBg, ui.Theme.ContrastFg = oldBackground, oldForeground
	return dimensions
}

func desktopSurface(
	gtx layout.Context,
	background color.NRGBA,
	radius int,
	padding unit.Dp,
	child layout.Widget,
) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			fillDesktop(gtx, background, radius)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(padding).Layout(gtx, child)
		},
	)
}

func fillDesktop(gtx layout.Context, background color.NRGBA, radius int) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 { size = gtx.Constraints.Min }
	rect := image.Rectangle{Max: size}
	shape := clip.Rect(rect).Op()
	if radius > 0 {
		shape = clip.UniformRRect(rect, gtx.Dp(unit.Dp(radius))).Op(gtx.Ops)
		paint.FillShape(gtx.Ops, background, shape)
		return
	}
	paint.FillShape(gtx.Ops, background, shape)
}

func severityColor(severity Severity) color.NRGBA {
	switch severity {
	case SeverityCritical: return desktopCoral
	case SeverityHigh: return desktopAmber
	case SeverityMedium: return desktopTeal
	default: return desktopMuted
	}
}

func stringsUpper(value string) string {
	out := []rune(value)
	for index, current := range out {
		if current >= 'a' && current <= 'z' { out[index] = current - ('a' - 'A') }
	}
	return string(out)
}
