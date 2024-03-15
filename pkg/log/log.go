import (
	"fmt"
)

const (
    ColorReset  = "\033[0m"
    ColorRed    = "\033[31m"
    ColorGreen  = "\033[32m"
    ColorBlue   = "\033[34m"
    ColorYellow = "\033[33m"
    ColorWhite  = "\033[37m"
)

func Info(msg string, args ...interface{}) {
  formatted := fmt.Sprintf(format, args...)
  fmt.Println(ColorWhite, formatted, ColorReset)
}

func Wait(format string, args ...interface{}) {
  formatted := fmt.Sprintf(format, args...) + "..."
  fmt.Println(ColorBlue, formatted, ColorReset)
}

func Good(msg string, args ...interface{}) {
  formatted := fmt.Sprintf(format, args...)
  fmt.Println(ColorGreen, formatted, ColorReset)
}

func Warn(msg string, args ...interface{}) {
  formatted := fmt.Sprintf(format, args...)
  fmt.Println(ColorYellow, formatted, ColorReset)
}

func Error(msg string, args ...interface{}) {
  formatted := fmt.Sprintf(format, args...)
  fmt.Println(ColorRed, formatted, ColorReset)
}
