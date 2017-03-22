// +build go1.8

package profile

import "runtime"

var setMutexProfileFraction = runtime.SetMutexProfileFraction
