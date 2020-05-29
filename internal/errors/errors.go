package errors

import (
	base "errors"
	"fmt"
	"reflect"
	"runtime"

	pkgerr "github.com/pkg/errors"
)

// The maximum number of stack frames on any error.
const maxStackDepth = 100

// traced is an error with an attached stack trace
type traced struct {
	err    error
	stack  []uintptr
	frames []stackFrame
	prefix string
}

func (e *traced) Error() string {
	msg := e.err.Error()
	if e.prefix != "" {
		msg = fmt.Sprintf("%s: %s", e.prefix, msg)
	}
	return msg
}

func (e *traced) Unwrap() error {
	return e.err
}

func (e *traced) String() string {
	return e.TypeName() + " " + e.Error()
}

// Trace returns the call stack formatted the same way that go does in runtime/debug.Stack()
func (e *traced) Trace() string {
	trace := ""
	for _, frame := range e.StackFrames() {
		trace += frame.String()
	}
	return trace
}

// StackFrames returns an array of frames containing information about the stack.
// This function must exist with this name for Sentry to report the correct trace.
// See extractReflectedStacktraceMethod() in github.com/getsentry/sentry-go/stacktrace.go
func (e *traced) StackFrames() []stackFrame {
	if e.frames == nil {
		e.frames = make([]stackFrame, len(e.stack))
		for i, pc := range e.stack {
			e.frames[i] = *newStackFrame(pc)
		}
	}

	return e.frames
}

// FullTrace returns a string that contains both the error message and the call stack.
func (e *traced) FullTrace() string {
	return e.String() + "\n" + e.Trace()
}

// TypeName returns the type this error. e.g. *errors.stringError.
func (e *traced) TypeName() string {
	return reflect.TypeOf(e.err).String()
}

// Err returns an error with stack trace
func Err(e interface{}, fmtParams ...interface{}) error {
	return wrap(1, e, fmtParams...)
}

// wrap intelligently creates/handles errors, while preserving the stack trace.
// Compatible with errors from github.com/pkg/errors.
// The skip parameter indicates how far up the stack to start the stacktrace. 0 starts from
// the function that calls wrap(), 1 from that function's caller, etc.
func wrap(skip int, e interface{}, fmtParams ...interface{}) *traced {
	if e == nil {
		return nil
	}

	var err error

	switch typed := e.(type) {
	case *traced:
		return typed
	case error:
		err = typed
	case string:
		err = fmt.Errorf(typed, fmtParams...)
	default:
		err = fmt.Errorf("%+v", typed)
	}

	var stack []uintptr
	if withStack, ok := e.(interface{ StackTrace() pkgerr.StackTrace }); ok { // interop with pkg/errors stack
		// get their stacktrace
		pkgStack := withStack.StackTrace()
		stack = make([]uintptr, len(pkgStack))
		for i, f := range pkgStack {
			stack[i] = uintptr(f)
		}
	} else {
		stack = make([]uintptr, maxStackDepth)
		length := runtime.Callers(2+skip, stack[:])
		stack = stack[:length]
	}

	return &traced{
		err:   err,
		stack: stack,
	}
}

// Is, As, and Unwrap call through to the builtin errors functions with the same name
func Is(err, target error) bool             { return base.Is(err, target) }
func As(err error, target interface{}) bool { return base.As(err, target) }
func Unwrap(err error) error                { return base.Unwrap(err) }

// Prefix prefixes the message of the error with the given string
func Prefix(prefix string, err interface{}) error {
	if err == nil {
		return nil
	}

	e := wrap(1, err)

	if e.prefix != "" {
		prefix = fmt.Sprintf("%s: %s", prefix, e.prefix)
	}
	e.prefix = prefix

	return e
}

// Trace returns the stack trace
func Trace(err error) string {
	if err == nil {
		return ""
	}
	return wrap(1, err).Trace()
}

// FullTrace returns the error type, message, and stack trace
func FullTrace(err error) string {
	if err == nil {
		return ""
	}
	return wrap(1, err).FullTrace()
}

// Base returns a simple error with no stack trace attached
func Base(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}

// HasTrace checks if error has a trace attached
func HasTrace(err error) bool {
	_, ok := err.(*traced)
	return ok
}

/*
Recover is similar to the bulitin `recover()`, except it includes a stack trace as well
Since `recover()` only works when called inside a deferred function (but not any function
called by it), you should call Recover() as follows

	err := func() (e error) {
		defer errors.Recover(&e)
		funcThatMayPanic()
		return e
	}()

*/
func Recover(e *error) {
	p := recover()
	if p == nil {
		return
	}

	err, ok := p.(error)
	if !ok {
		err = fmt.Errorf("%v", p)
	}

	stack := make([]uintptr, maxStackDepth)
	length := runtime.Callers(4, stack[:])
	stack = stack[:length]

	*e = &traced{
		err:    err,
		stack:  stack,
		prefix: "panic",
	}
}
