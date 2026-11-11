package rpc

import (
	"errors"
	"log"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/admin"
	"github.com/wargasipil/hai-tukang/internal/catalog"
	"github.com/wargasipil/hai-tukang/internal/penalties"
	"github.com/wargasipil/hai-tukang/internal/photos"
	"github.com/wargasipil/hai-tukang/internal/push"
	"github.com/wargasipil/hai-tukang/internal/requests"
	"github.com/wargasipil/hai-tukang/internal/stores"
	"github.com/wargasipil/hai-tukang/internal/verify"
	"github.com/wargasipil/hai-tukang/internal/visits"
)

// knownErrors maps domain errors to a Connect code and the stable reason
// the frontend translates. A new reason needs a message in every frontend
// language (typescripts/hai-tukang/src/i18n).
var knownErrors = []struct {
	err    error
	code   connect.Code
	reason string
}{
	{accounts.ErrInvalidRole, connect.CodeInvalidArgument, "invalid_role"},
	{accounts.ErrInvalidName, connect.CodeInvalidArgument, "invalid_name"},
	{accounts.ErrInvalidPhone, connect.CodeInvalidArgument, "invalid_phone"},
	{accounts.ErrWeakPassword, connect.CodeInvalidArgument, "weak_password"},
	{accounts.ErrPhoneTaken, connect.CodeAlreadyExists, "phone_taken"},
	{accounts.ErrInvalidCredentials, connect.CodeUnauthenticated, "invalid_credentials"},
	{accounts.ErrTermsNotAccepted, connect.CodeFailedPrecondition, "terms_not_accepted"},
	{accounts.ErrGoogleNotConfigured, connect.CodeFailedPrecondition, "google_not_configured"},
	{accounts.ErrInvalidGoogleToken, connect.CodeUnauthenticated, "invalid_google_token"},
	{accounts.ErrGoogleTaken, connect.CodeAlreadyExists, "google_taken"},
	{accounts.ErrNoSession, connect.CodeUnauthenticated, "unauthorized"},
	{accounts.ErrNotRepairman, connect.CodePermissionDenied, "forbidden"},
	{accounts.ErrInvalidAbout, connect.CodeInvalidArgument, "invalid_about"},
	{catalog.ErrInvalidService, connect.CodeInvalidArgument, "invalid_service"},
	{catalog.ErrInvalidArea, connect.CodeInvalidArgument, "invalid_area"},
	{requests.ErrNotCustomer, connect.CodePermissionDenied, "forbidden"},
	{requests.ErrNotRepairman, connect.CodePermissionDenied, "forbidden"},
	{requests.ErrInvalidDescription, connect.CodeInvalidArgument, "invalid_description"},
	{requests.ErrInvalidMessage, connect.CodeInvalidArgument, "invalid_message"},
	{requests.ErrNotFound, connect.CodeNotFound, "not_found"},
	{requests.ErrAlreadyResponded, connect.CodeAlreadyExists, "already_responded"},
	{requests.ErrProfileRequired, connect.CodeFailedPrecondition, "profile_required"},
	{verify.ErrInvalidCode, connect.CodeInvalidArgument, "invalid_code"},
	{verify.ErrCodeExpired, connect.CodeFailedPrecondition, "code_expired"},
	{verify.ErrTooManyAttempts, connect.CodeResourceExhausted, "too_many_attempts"},
	{verify.ErrResendTooSoon, connect.CodeResourceExhausted, "resend_too_soon"},
	{verify.ErrTooManyCodes, connect.CodeResourceExhausted, "too_many_codes"},
	{verify.ErrNotSent, connect.CodeUnavailable, "code_not_sent"},
	{visits.ErrNotFound, connect.CodeNotFound, "not_found"},
	{visits.ErrNotCustomer, connect.CodePermissionDenied, "forbidden"},
	{visits.ErrNotRepairman, connect.CodePermissionDenied, "forbidden"},
	{visits.ErrNotResponded, connect.CodeFailedPrecondition, "not_responded"},
	{visits.ErrInvalidWindow, connect.CodeInvalidArgument, "invalid_window"},
	{visits.ErrVisitExists, connect.CodeAlreadyExists, "visit_exists"},
	{visits.ErrNotYourTurn, connect.CodeFailedPrecondition, "not_your_turn"},
	{visits.ErrVisitChanged, connect.CodeFailedPrecondition, "visit_changed"},
	{visits.ErrWrongArrivalCode, connect.CodeInvalidArgument, "wrong_arrival_code"},
	{visits.ErrArrivalLocked, connect.CodeResourceExhausted, "arrival_locked"},
	{visits.ErrTooEarly, connect.CodeFailedPrecondition, "no_show_too_early"},
	{push.ErrInvalidSubscription, connect.CodeInvalidArgument, "invalid_subscription"},
	{penalties.ErrSuspended, connect.CodePermissionDenied, "suspended"},
	{visits.ErrRepairmanUnavailable, connect.CodeFailedPrecondition, "repairman_unavailable"},
	{visits.ErrAbsentNotNow, connect.CodeFailedPrecondition, "absent_not_now"},
	{visits.ErrDisputeClosed, connect.CodeFailedPrecondition, "dispute_closed"},
	{visits.ErrInvalidNote, connect.CodeInvalidArgument, "invalid_note"},
	{admin.ErrNotAdmin, connect.CodePermissionDenied, "forbidden"},
	{admin.ErrInvalidReason, connect.CodeInvalidArgument, "invalid_reason"},
	{admin.ErrInvalidDays, connect.CodeInvalidArgument, "invalid_days"},
	{admin.ErrNotFound, connect.CodeNotFound, "not_found"},
	{admin.ErrNotDisputed, connect.CodeFailedPrecondition, "not_disputed"},
	{admin.ErrNotClosed, connect.CodeFailedPrecondition, "not_closed"},
	{admin.ErrAlreadySuspended, connect.CodeFailedPrecondition, "already_suspended"},
	{admin.ErrNotSuspended, connect.CodeFailedPrecondition, "not_suspended"},
	{stores.ErrNotStore, connect.CodePermissionDenied, "forbidden"},
	{stores.ErrProfileRequired, connect.CodeFailedPrecondition, "store_profile_required"},
	{stores.ErrNotFound, connect.CodeNotFound, "not_found"},
	{stores.ErrTooManyPhotos, connect.CodeFailedPrecondition, "too_many_photos"},
	{stores.ErrInvalidStoreName, connect.CodeInvalidArgument, "invalid_store_name"},
	{stores.ErrInvalidAddress, connect.CodeInvalidArgument, "invalid_address"},
	{stores.ErrNoHandover, connect.CodeInvalidArgument, "no_handover"},
	{stores.ErrInvalidDeliveryFee, connect.CodeInvalidArgument, "invalid_delivery_fee"},
	{stores.ErrInvalidDeliveryAreas, connect.CodeInvalidArgument, "invalid_delivery_areas"},
	{stores.ErrInvalidProductName, connect.CodeInvalidArgument, "invalid_product_name"},
	{stores.ErrInvalidPrice, connect.CodeInvalidArgument, "invalid_price"},
	{stores.ErrInvalidUnit, connect.CodeInvalidArgument, "invalid_unit"},
	{stores.ErrInvalidProductDescription, connect.CodeInvalidArgument, "invalid_product_description"},
	{photos.ErrInvalid, connect.CodeInvalidArgument, "invalid_photo"},
	{photos.ErrTooLarge, connect.CodeInvalidArgument, "photo_too_large"},
}

// rpcError turns a domain error into a Connect error with an ErrorDetail.
// Anything unexpected is logged and answered as internal, without details.
func rpcError(err error) error {
	for _, known := range knownErrors {
		if errors.Is(err, known.err) {
			return withReason(known.code, known.reason, err)
		}
	}
	log.Printf("internal error: %v", err)
	return withReason(connect.CodeInternal, "internal", errors.New("internal error"))
}

func withReason(code connect.Code, reason string, err error) error {
	connectErr := connect.NewError(code, err)
	if detail, detailErr := connect.NewErrorDetail(&v1.ErrorDetail{Reason: reason}); detailErr == nil {
		connectErr.AddDetail(detail)
	}
	return connectErr
}

// Reason returns the ErrorDetail reason of an error from the services, or "".
func Reason(err error) string {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return ""
	}
	for _, detail := range connectErr.Details() {
		if value, err := detail.Value(); err == nil {
			if d, ok := value.(*v1.ErrorDetail); ok {
				return d.GetReason()
			}
		}
	}
	return ""
}
