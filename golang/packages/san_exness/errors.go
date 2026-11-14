package san_exness

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrInvalidConfig: a Config value is missing or malformed.
	ErrInvalidConfig = errors.New("exness: invalid config")
	// ErrInvalidRequest: a request is refused before it is sent (a missing
	// field, a number that is not a decimal string, a bad id).
	ErrInvalidRequest = errors.New("exness: invalid request")
	// ErrBadAccessPoint: discovery answered without a usable Access Point.
	ErrBadAccessPoint = errors.New("exness: discovery returned no valid access point")
)

// Public error codes, from the API's PublicApiErrorCode (version 1.0.2)
// and the troubleshooting guide (AUTH_RATE_LIMIT).
const (
	CodeSystemRateLimit                   = 1
	CodeSystemTemporaryUnavailable        = 2
	CodeSystemInternalError               = 3
	CodeAuthInvalidAPIKey                 = 1000
	CodeAuthInvalidSignature              = 1001
	CodeAuthPermissionDenied              = 1002
	CodeAuthRateLimit                     = 1003
	CodeAccountNotFound                   = 2000
	CodeAccountDisabled                   = 2001
	CodeAccountCloseOnly                  = 2002
	CodeRequestInvalid                    = 3000
	CodeRequestInvalidParameters          = 3001
	CodeRequestInvalidPrice               = 3002
	CodeRequestInvalidTimeframe           = 3003
	CodeRequestInvalidTimeRange           = 3004
	CodeRequestInvalidCount               = 3005
	CodeRequestInvalidLimit               = 3006
	CodeRequestInvalidCursor              = 3007
	CodeRequestInvalidPriceType           = 3008
	CodeRequestInvalidFilterCombination   = 3009
	CodeRequestUnsupportedMediaType       = 3010
	CodeRequestMethodNotAllowed           = 3011
	CodeRequestOrderNotFound              = 3012
	CodeRequestPositionNotFound           = 3013
	CodeRequestOperationNotFound          = 3014
	CodeRequestRateLimit                  = 3015
	CodeMarketInstrumentNotFound          = 4000
	CodeMarketTradeDisabled               = 4001
	CodeMarketCloseOnly                   = 4002
	CodeMarketSessionClosed               = 4003
	CodeMarketNoQuotes                    = 4004
	CodeTradingRuleInvalidVolume          = 5000
	CodeTradingRuleVolumeTooSmall         = 5001
	CodeTradingRuleVolumeTooLarge         = 5002
	CodeTradingRuleInvalidVolumeStep      = 5003
	CodeTradingRuleInvalidRemainingVolume = 5004
	CodeTradingRuleInvalidPriceLevels     = 5005
	CodeTradingRuleInsufficientMargin     = 5006
	CodeTradingRuleMarginLevelTooLow      = 5007
	CodeTradingRuleOperationRestricted    = 5008
	CodeTradingRuleTooManyPendingOrders   = 5009
	CodeTradingRuleTooManyOpenPositions   = 5010
	CodeExecutionRequote                  = 6000
	CodeExecutionRejected                 = 6001
)

// CodeNames are the codes' stable names, the API's error_message values.
var CodeNames = map[int]string{
	CodeSystemRateLimit:                   "SYSTEM_RATE_LIMIT",
	CodeSystemTemporaryUnavailable:        "SYSTEM_TEMPORARY_UNAVAILABLE",
	CodeSystemInternalError:               "SYSTEM_INTERNAL_ERROR",
	CodeAuthInvalidAPIKey:                 "AUTH_INVALID_API_KEY",
	CodeAuthInvalidSignature:              "AUTH_INVALID_SIGNATURE",
	CodeAuthPermissionDenied:              "AUTH_PERMISSION_DENIED",
	CodeAuthRateLimit:                     "AUTH_RATE_LIMIT",
	CodeAccountNotFound:                   "ACCOUNT_NOT_FOUND",
	CodeAccountDisabled:                   "ACCOUNT_DISABLED",
	CodeAccountCloseOnly:                  "ACCOUNT_CLOSE_ONLY",
	CodeRequestInvalid:                    "REQUEST_INVALID",
	CodeRequestInvalidParameters:          "REQUEST_INVALID_PARAMETERS",
	CodeRequestInvalidPrice:               "REQUEST_INVALID_PRICE",
	CodeRequestInvalidTimeframe:           "REQUEST_INVALID_TIMEFRAME",
	CodeRequestInvalidTimeRange:           "REQUEST_INVALID_TIME_RANGE",
	CodeRequestInvalidCount:               "REQUEST_INVALID_COUNT",
	CodeRequestInvalidLimit:               "REQUEST_INVALID_LIMIT",
	CodeRequestInvalidCursor:              "REQUEST_INVALID_CURSOR",
	CodeRequestInvalidPriceType:           "REQUEST_INVALID_PRICE_TYPE",
	CodeRequestInvalidFilterCombination:   "REQUEST_INVALID_FILTER_COMBINATION",
	CodeRequestUnsupportedMediaType:       "REQUEST_UNSUPPORTED_MEDIA_TYPE",
	CodeRequestMethodNotAllowed:           "REQUEST_METHOD_NOT_ALLOWED",
	CodeRequestOrderNotFound:              "REQUEST_ORDER_NOT_FOUND",
	CodeRequestPositionNotFound:           "REQUEST_POSITION_NOT_FOUND",
	CodeRequestOperationNotFound:          "REQUEST_OPERATION_NOT_FOUND",
	CodeRequestRateLimit:                  "REQUEST_RATE_LIMIT",
	CodeMarketInstrumentNotFound:          "MARKET_INSTRUMENT_NOT_FOUND",
	CodeMarketTradeDisabled:               "MARKET_TRADE_DISABLED",
	CodeMarketCloseOnly:                   "MARKET_CLOSE_ONLY",
	CodeMarketSessionClosed:               "MARKET_SESSION_CLOSED",
	CodeMarketNoQuotes:                    "MARKET_NO_QUOTES",
	CodeTradingRuleInvalidVolume:          "TRADING_RULE_INVALID_VOLUME",
	CodeTradingRuleVolumeTooSmall:         "TRADING_RULE_VOLUME_TOO_SMALL",
	CodeTradingRuleVolumeTooLarge:         "TRADING_RULE_VOLUME_TOO_LARGE",
	CodeTradingRuleInvalidVolumeStep:      "TRADING_RULE_INVALID_VOLUME_STEP",
	CodeTradingRuleInvalidRemainingVolume: "TRADING_RULE_INVALID_REMAINING_VOLUME",
	CodeTradingRuleInvalidPriceLevels:     "TRADING_RULE_INVALID_PRICE_LEVELS",
	CodeTradingRuleInsufficientMargin:     "TRADING_RULE_INSUFFICIENT_MARGIN",
	CodeTradingRuleMarginLevelTooLow:      "TRADING_RULE_MARGIN_LEVEL_TOO_LOW",
	CodeTradingRuleOperationRestricted:    "TRADING_RULE_OPERATION_RESTRICTED",
	CodeTradingRuleTooManyPendingOrders:   "TRADING_RULE_TOO_MANY_PENDING_ORDERS",
	CodeTradingRuleTooManyOpenPositions:   "TRADING_RULE_TOO_MANY_OPEN_POSITIONS",
	CodeExecutionRequote:                  "EXECUTION_REQUOTE",
	CodeExecutionRejected:                 "EXECUTION_REJECTED",
}

// APIError is a REST request refused before Exness accepted it.
type APIError struct {
	Status     int           // HTTP status
	Code       int           // Exness code; 0 when the body was not Exness JSON (an edge layer's 403)
	Message    string        // error_message, e.g. "REQUEST_INVALID_PARAMETERS"
	RetryAfter time.Duration // from Retry-After or retry_after, on rate limits
	Body       string        // the start of a body that was not Exness JSON
}

func (e *APIError) Error() string {
	if e.Code == 0 {
		if e.Body != "" {
			return fmt.Sprintf("exness: HTTP %d (not an API error: %q)", e.Status, e.Body)
		}
		return fmt.Sprintf("exness: HTTP %d", e.Status)
	}
	s := fmt.Sprintf("exness: HTTP %d: %s (%d)", e.Status, e.Message, e.Code)
	if e.RetryAfter > 0 {
		s += fmt.Sprintf(", retry after %s", e.RetryAfter)
	}
	return s
}

// OperationError is a trading operation Exness accepted (ACK) and then
// rejected or failed: the result in operation status or transaction_event.
type OperationError struct {
	OperationID string
	Status      string // "rejected" or "failed"
	Code        int
	Message     string
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("exness: operation %s %s: %s (%d)", e.OperationID, e.Status, e.Message, e.Code)
}
