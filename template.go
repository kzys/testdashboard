package main

import (
	"html/template"
	"io"

	"github.com/labstack/echo"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	c.Echo().Logger.Debugf("name = %s", name)
	return t.templates.ExecuteTemplate(w, name, data)
}
