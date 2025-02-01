package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

type GuiContext struct {
	filename string
}

func main() {
	// c := GuiContext{}

	a := app.New()
	w := a.NewWindow("goroom recording studio design software")
	w.Resize(fyne.Size{
		Width:  600,
		Height: 600,
	})

	hello := widget.NewLabel("goroom recording studio design")
	w.SetContent(container.NewVBox(
		hello,
		widget.NewButton("Import room model", func() {
			fd := dialog.NewFileOpen(func(f fyne.URIReadCloser, err error) {
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				if f == nil {
					return
				}
				hello.SetText(fmt.Sprintf("File: %s", f.URI().Path()))
			}, w)

			cwd, err := os.Getwd()
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			luri, err := storage.ListerForURI(storage.NewFileURI(cwd))
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			fd.SetLocation(luri)
			fd.Show()
		}),
	))

	w.ShowAndRun()
}
