package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/nsf/jsondiff"
)

var testConsoleOptions = jsondiff.Options{
	Added:   jsondiff.Tag{Begin: "++", End: "++"},
	Removed: jsondiff.Tag{Begin: "--", End: "--"},
	Changed: jsondiff.Tag{Begin: "[[", End: "]]"},
	Indent:  "    ",
}

// JSONCompact removes insignificant space characters from a JSON string.
// It helps compare JSON strings without worrying about whitespace differences.
func JSONCompact(jsonStr string) string {
	dst := &bytes.Buffer{}
	err := json.Compact(dst, []byte(jsonStr))
	if err != nil {
		panic(err)
	}
	return dst.String()
}

// GetJSONDiffLog compares two JSON strings or bytes and returns `false` if they match
// or `true` if they don't, plus difference log in a text format suitable for console output.
func GetJSONDiffLog(expected, actual interface{}) (bool, string) {
	diff, diffLog := jsondiff.Compare(toBytes(expected), toBytes(actual), &testConsoleOptions)
	differs := diff != jsondiff.FullMatch

	if !differs {
		return false, ""
	}

	indent := "\t\t"
	diffIndented := regexp.MustCompile("(?m)^").ReplaceAll([]byte(diffLog), []byte("\t"+indent))[len(indent)+1:]
	return differs, fmt.Sprintf("\n\tError:"+indent+"JSON not equal\n\tDiff:"+indent+"%s", diffIndented)
}

// AssertEqualJSON is assert.Equal equivalent for JSON with better comparison and diff output.
func AssertEqualJSON(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	differs, diffLog := GetJSONDiffLog(expected, actual)
	if !differs {
		return true
	}
	indent := "\t\t"
	msg := messageFromMsgAndArgs(msgAndArgs...)
	if len(msg) > 0 {
		t.Errorf(diffLog+"\n\tMessages:"+indent+"%s", msg)
	} else {
		t.Errorf(diffLog)
	}
	return false
}

func toBytes(v interface{}) []byte {
	switch s := v.(type) {
	case string:
		return []byte(s)
	case []byte:
		return s
	default:
		panic(fmt.Sprintf("cannot convert %T to byte slice", v))
	}
}

// copied from assert.Fail()
func messageFromMsgAndArgs(msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 || msgAndArgs == nil {
		return ""
	}
	if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0]
		if msgAsStr, ok := msg.(string); ok {
			return msgAsStr
		}
		return fmt.Sprintf("%+v", msg)
	}
	if len(msgAndArgs) > 1 {
		return fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
	}
	return ""
}
