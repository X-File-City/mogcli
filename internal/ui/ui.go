package ui

import (
	"context"
	"fmt"
	"io"
	"os"
)

type Options struct {
	Stdout io.Writer
	Stderr io.Writer
}

type UI struct {
	out *Printer
	err *Printer
}

func New(opts Options) *UI {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	return &UI{
		out: &Printer{w: opts.Stdout},
		err: &Printer{w: opts.Stderr},
	}
}

func (u *UI) Out() *Printer { return u.out }
func (u *UI) Err() *Printer { return u.err }

type Printer struct {
	w io.Writer
}

func (p *Printer) line(s string) {
	_, _ = io.WriteString(p.w, s+"\n")
}

func (p *Printer) Print(msg string) {
	_, _ = io.WriteString(p.w, msg)
}

func (p *Printer) Printf(format string, args ...any) { p.line(fmt.Sprintf(format, args...)) }
func (p *Printer) Println(msg string)                { p.line(msg) }
func (p *Printer) Successf(format string, args ...any) {
	p.line(fmt.Sprintf(format, args...))
}
func (p *Printer) Error(msg string)                  { p.line(msg) }
func (p *Printer) Errorf(format string, args ...any) { p.Error(fmt.Sprintf(format, args...)) }

type ctxKey struct{}

func WithUI(ctx context.Context, u *UI) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func FromContext(ctx context.Context) *UI {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return nil
	}
	u, _ := v.(*UI)
	return u
}
