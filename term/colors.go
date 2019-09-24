package term

import (
	"fmt"
)

type colorValue int

// Color codes
const (
	CBlack        colorValue = 30
	CRed          colorValue = 31
	CGreen        colorValue = 32
	CYellow       colorValue = 33
	CBlue         colorValue = 34
	CMagenta      colorValue = 35
	CCyan         colorValue = 36
	CLightGray    colorValue = 37
	CDarkGray     colorValue = 90
	CLightRed     colorValue = 91
	CLightGreen   colorValue = 92
	CLightYellow  colorValue = 93
	CLightBlue    colorValue = 94
	CLightMagenta colorValue = 95
	CLightCyan    colorValue = 96
	CWhite        colorValue = 97
)

// Colored wraps message into esc sequences to make it colored
func Colored(message string, c colorValue, bold bool) string {
	bstr := ""
	if bold {
		bstr = ";1"
	}
	return fmt.Sprintf("\033[%d%sm%s\033[0m", c, bstr, message)
}

// Blue returns message colored with light blue color
func Blue(message string) string {
	return Colored(message, CLightBlue, false)
}

// Red returns message colored with light red color
func Red(message string) string {
	return Colored(message, CLightRed, false)
}

// Green returns message colored with light green color
func Green(message string) string {
	return Colored(message, CLightGreen, false)
}

// Yellow returns message colored with light yellow color
func Yellow(message string) string {
	return Colored(message, CLightYellow, false)
}

// Cyan returns message colored with light cyan color
func Cyan(message string) string {
	return Colored(message, CLightCyan, false)
}

// Errorf prints a red-colored formatted error message
func Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Print(Red(message))
}

// Successf prints a green-colored formatted message
func Successf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Print(Green(message))
}

// Warnf prints a yellow-colored formatted warning message
func Warnf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Print(Yellow(message))
}
