package command

// ExitCodes is the closed set of process exit codes codelens is allowed to
// produce, per the exit-code taxonomy (ADR 0002, BSD sysexits.h range):
//
//	0  success (including an empty-but-valid result)
//	64 usage error   (EX_USAGE)
//	65 data error    (EX_DATAERR)
//	70 internal fault (EX_SOFTWARE)
//	74 I/O error     (EX_IOERR)
//
// It is declared as data so the exit-code conformance test reads it instead of
// re-deriving the set by unioning every command's declared codes.
var ExitCodes = []int{0, 64, 65, 70, 74}
