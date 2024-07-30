package busadminnotifier

import (
	"fmt"
	"github.com/jroedel/tmpcontrol/foundation/clienttoserverapi"
	"github.com/jroedel/tmpcontrol/foundation/sms"
	"regexp"
)

type AdminNotifier struct {
	sms       *sms.Sms
	adminCell string
	api       *clienttoserverapi.Client
}

func New(sms *sms.Sms, adminCell string, api *clienttoserverapi.Client) (*AdminNotifier, error) {
	if sms == nil && adminCell == "" && api == nil {
		return nil, fmt.Errorf("no SMS or API")
	}
	an := &AdminNotifier{
		sms:       sms,
		adminCell: adminCell,
		api:       api,
	}
	//they should both be zero or both be set to prevent unexpected behavior
	if (sms == nil) != (adminCell == "") {
		return nil, fmt.Errorf("sms: both sms and admin cell must be set to enable sms")
	}
	if adminCell != "" {
		const cellValidationRegex = `(?m)\+1\d{10,10}`
		regex := regexp.MustCompile(cellValidationRegex)
		if !regex.MatchString(adminCell) {
			return nil, fmt.Errorf("invalid admin cell: %s", adminCell)
		}
	}
	return an, nil
}

func (an *AdminNotifier) NotifyAdmin(message string, urgency clienttoserverapi.Urgency) {
	notified := false
	var err error
	//TODO we may be doing multiple API calls here, for efficiency we should do them async
	if an.sms != nil {
		//TODO retry a few times?
		err = an.sms.Send(an.adminCell, message)
		if err == nil {
			notified = true
		}
	}
	if an.api != nil {
		msg := an.api.NewNotifyApiMessage()
		msg.Message = message
		msg.Urgency = urgency
		msg.HasAdminBeenNotified = notified
		_ = an.api.Notify(msg)
	}
	//TODO the api was healthy when we constructed it, should we do a few retries async?
}
