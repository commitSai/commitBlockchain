package types

import (
	"fmt"
	"strings"
	
	cmn "github.com/tendermint/tendermint/libs/common"
	
	"github.com/comdex-blockchain/wire"
	abci "github.com/tendermint/tendermint/abci/types"
)

// ABCICodeType - combined codetype / codespace
type ABCICodeType uint32

// CodeType - code identifier within codespace
type CodeType uint16

// CodespaceType - codespace identifier
type CodespaceType uint16

// IsOK - is everything okay?
func (code ABCICodeType) IsOK() bool {
	if code == ABCICodeOK {
		return true
	}
	return false
}

// ToABCICode get the abci code from the local code and codespace
func ToABCICode(space CodespaceType, code CodeType) ABCICodeType {
	// TODO: Make Tendermint more aware of codespaces.
	if space == CodespaceRoot && code == CodeOK {
		return ABCICodeOK
	}
	return ABCICodeType((uint32(space) << 16) | uint32(code))
}

// SDK error codes
const (
	// ABCI error codes
	ABCICodeOK ABCICodeType = 0
	
	// Base error codes
	CodeOK                     CodeType = 0
	CodeInternal               CodeType = 1
	CodeTxDecode               CodeType = 2
	CodeInvalidSequence        CodeType = 3
	CodeUnauthorized           CodeType = 4
	CodeInsufficientFunds      CodeType = 5
	CodeUnknownRequest         CodeType = 6
	CodeInvalidAddress         CodeType = 7
	CodeInvalidPubKey          CodeType = 8
	CodeUnknownAddress         CodeType = 9
	CodeInsufficientCoins      CodeType = 10
	CodeInvalidCoins           CodeType = 11
	CodeOutOfGas               CodeType = 12
	CodeMemoTooLarge           CodeType = 13
	CodeFeedbackCannotRegister CodeType = 14
	
	// CodespaceRoot is a codespace for error codes in this file only.
	// Notice that 0 is an "unset" codespace, which can be overridden with
	// Error.WithDefaultCodespace().
	CodespaceUndefined CodespaceType = 0
	CodespaceRoot      CodespaceType = 1
	
	// Maximum reservable codespace (2^16 - 1)
	MaximumCodespace CodespaceType = 65535
)

func unknownCodeMsg(code CodeType) string {
	return fmt.Sprintf("unknown code %d", code)
}

// CodeToDefaultMsg :
// NOTE: Don't stringer this, we'll put better messages in later.
// nolint: gocyclo
func CodeToDefaultMsg(code CodeType) string {
	switch code {
	case CodeInternal:
		return "internal error"
	case CodeTxDecode:
		return "tx parse error"
	case CodeInvalidSequence:
		return "invalid sequence"
	case CodeUnauthorized:
		return "unauthorized"
	case CodeInsufficientFunds:
		return "insufficient funds"
	case CodeUnknownRequest:
		return "unknown request"
	case CodeInvalidAddress:
		return "invalid address"
	case CodeInvalidPubKey:
		return "invalid pubkey"
	case CodeUnknownAddress:
		return "unknown address"
	case CodeInsufficientCoins:
		return "insufficient coins"
	case CodeInvalidCoins:
		return "invalid coins"
	case CodeOutOfGas:
		return "out of gas"
	case CodeMemoTooLarge:
		return "memo too large"
	case CodeFeedbackCannotRegister:
		return "already given a feedback"
	default:
		return unknownCodeMsg(code)
	}
}

// --------------------------------------------------------------------------------
// All errors are created via constructors so as to enable us to hijack them
// and inject stack traces if we really want to.

// ErrInternal :
// nolint
func ErrInternal(msg string) Error {
	return newErrorWithRootCodespace(CodeInternal, msg)
}

// ErrTxDecode :
func ErrTxDecode(msg string) Error {
	return newErrorWithRootCodespace(CodeTxDecode, msg)
}

// ErrInvalidSequence :
func ErrInvalidSequence(msg string) Error {
	return newErrorWithRootCodespace(CodeInvalidSequence, msg)
}

// ErrUnauthorized :
func ErrUnauthorized(msg string) Error {
	return newErrorWithRootCodespace(CodeUnauthorized, msg)
}

// ErrInsufficientFunds :
func ErrInsufficientFunds(msg string) Error {
	return newErrorWithRootCodespace(CodeInsufficientFunds, msg)
}

// ErrUnknownRequest :
func ErrUnknownRequest(msg string) Error {
	return newErrorWithRootCodespace(CodeUnknownRequest, msg)
}

// ErrInvalidAddress :
func ErrInvalidAddress(msg string) Error {
	return newErrorWithRootCodespace(CodeInvalidAddress, msg)
}

// ErrUnknownAddress :
func ErrUnknownAddress(msg string) Error {
	return newErrorWithRootCodespace(CodeUnknownAddress, msg)
}

// ErrInvalidPubKey :
func ErrInvalidPubKey(msg string) Error {
	return newErrorWithRootCodespace(CodeInvalidPubKey, msg)
}

// ErrInsufficientCoins :
func ErrInsufficientCoins(msg string) Error {
	return newErrorWithRootCodespace(CodeInsufficientCoins, msg)
}

// ErrInvalidCoins :
func ErrInvalidCoins(msg string) Error {
	return newErrorWithRootCodespace(CodeInvalidCoins, msg)
}

// ErrOutOfGas :
func ErrOutOfGas(msg string) Error {
	return newErrorWithRootCodespace(CodeOutOfGas, msg)
}

// ErrMemoTooLarge :
func ErrMemoTooLarge(msg string) Error {
	return newErrorWithRootCodespace(CodeMemoTooLarge, msg)
}

// ErrFeedbackCannotRegister :
func ErrFeedbackCannotRegister(msg string) Error {
	return newErrorWithRootCodespace(CodeFeedbackCannotRegister, msg)
}

// ----------------------------------------
// Error & sdkError

type cmnError = cmn.Error

// Error sdk Error type
type Error interface {
	// Implements cmn.Error
	// Error() string
	// Stacktrace() cmn.Error
	// Trace(offset int, format string, args ...interface{}) cmn.Error
	// Data() interface{}
	cmnError
	
	// convenience
	TraceSDK(format string, args ...interface{}) Error
	
	// set codespace
	WithDefaultCodespace(CodespaceType) Error
	
	Code() CodeType
	Codespace() CodespaceType
	ABCILog() string
	ABCICode() ABCICodeType
	Result() Result
	QueryResult() abci.ResponseQuery
}

// NewError - create an error.
func NewError(codespace CodespaceType, code CodeType, format string, args ...interface{}) Error {
	return newError(codespace, code, format, args...)
}

func newErrorWithRootCodespace(code CodeType, format string, args ...interface{}) *sdkError {
	return newError(CodespaceRoot, code, format, args...)
}

func newError(codespace CodespaceType, code CodeType, format string, args ...interface{}) *sdkError {
	if format == "" {
		format = CodeToDefaultMsg(code)
	}
	return &sdkError{
		codespace: codespace,
		code:      code,
		cmnError:  cmn.NewError(format, args...),
	}
}

type sdkError struct {
	codespace CodespaceType
	code      CodeType
	cmnError
}

// Implements Error.
func (err *sdkError) WithDefaultCodespace(cs CodespaceType) Error {
	codespace := err.codespace
	if codespace == CodespaceUndefined {
		codespace = cs
	}
	return &sdkError{
		codespace: cs,
		code:      err.code,
		cmnError:  err.cmnError,
	}
}

// Implements ABCIError.
func (err *sdkError) TraceSDK(format string, args ...interface{}) Error {
	err.Trace(1, format, args...)
	return err
}

// Implements ABCIError.
// Overrides err.Error.Error().
func (err *sdkError) Error() string {
	return fmt.Sprintf(`ERROR:
Codespace: %d
Code: %d
Message: %#v
`, err.codespace, err.code, parseCmnError(err.cmnError.Error()))
}

// Implements ABCIError.
func (err *sdkError) ABCICode() ABCICodeType {
	return ToABCICode(err.codespace, err.code)
}

// Implements Error.
func (err *sdkError) Codespace() CodespaceType {
	return err.codespace
}

// Implements Error.
func (err *sdkError) Code() CodeType {
	return err.code
}

// Implements ABCIError.
func (err *sdkError) ABCILog() string {
	cdc := wire.NewCodec()
	parsedErrMsg := parseCmnError(err.cmnError.Error())
	jsonErr := humanReadableError{
		Codespace: err.codespace,
		Code:      err.code,
		ABCICode:  err.ABCICode(),
		Message:   parsedErrMsg,
	}
	bz, er := cdc.MarshalJSON(jsonErr)
	if er != nil {
		panic(er)
	}
	stringifiedJSON := string(bz)
	return stringifiedJSON
}

func (err *sdkError) Result() Result {
	return Result{
		Code: err.ABCICode(),
		Log:  err.ABCILog(),
	}
}

// QueryResult allows us to return sdk.Error.QueryResult() in query responses
func (err *sdkError) QueryResult() abci.ResponseQuery {
	return abci.ResponseQuery{
		Code: uint32(err.ABCICode()),
		Log:  err.ABCILog(),
	}
}

func parseCmnError(err string) string {
	if idx := strings.Index(err, "{"); idx != -1 {
		err = err[idx+1 : len(err)-1]
	}
	return err
}

// nolint
type humanReadableError struct {
	Codespace CodespaceType `json:"codespace"`
	Code      CodeType      `json:"code"`
	ABCICode  ABCICodeType  `json:"abci_code"`
	Message   string        `json:"message"`
}