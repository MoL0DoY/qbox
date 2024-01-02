package models

import "io"

type Formatter interface {
	Render(writer io.Writer, device *DataDevice)
}
