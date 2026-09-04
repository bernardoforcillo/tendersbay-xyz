package codelist_test

import "errors"

func asError[T error](err error, target *T) bool { return errors.As(err, target) }
