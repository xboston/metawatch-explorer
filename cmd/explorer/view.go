package main

import (
	"io"

	"github.com/foolin/goview"
	"github.com/labstack/echo/v4"
)

// const templateEngineKey = "foolin-goview-echoview"

type ViewEngine struct {
	*goview.ViewEngine
}

func NewViewEngine(config goview.Config) *ViewEngine {
	return &ViewEngine{
		ViewEngine: goview.New(config),
	}
}

func (e *ViewEngine) Render(w io.Writer, name string, data interface{}, c echo.Context) error {

	if viewContext, isMap := data.(echo.Map); isMap {
		viewContext["echoReq"] = c.Request()
	}

	return e.RenderWriter(w, name, data)
}
