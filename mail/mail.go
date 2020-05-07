package mail

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/hiconvo/api/clients/mail"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/template"
)

const (
	_fromEmail = "robots@mail.convo.events"
	_fromName  = "Convo"
)

type Client struct {
	mail mail.Client
	tpl  *template.Client

	tplStrPasswordReset string
	tplStrVerifyEmail   string
	tplStrMergeAccounts string
}

func New(sender mail.Client, tpl *template.Client) *Client {
	return &Client{
		mail: sender,
		tpl:  tpl,

		tplStrPasswordReset: readStringFromFile("password-reset.txt"),
		tplStrVerifyEmail:   readStringFromFile("verify-email.txt"),
		tplStrMergeAccounts: readStringFromFile("merge-accounts.txt"),
	}
}

func (c *Client) SendPasswordResetEmail(u *model.User, magicLink string) error {
	plainText, html, err := c.tpl.RenderAdminEmail(&template.AdminEmail{
		Body:       c.tplStrPasswordReset,
		ButtonText: "Set password",
		MagicLink:  magicLink,
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    _fromName,
		FromEmail:   _fromEmail,
		ToName:      u.FullName,
		ToEmail:     u.Email,
		Subject:     "[convo] Set Password",
		TextContent: plainText,
		HTMLContent: html,
	}

	return c.mail.Send(email)
}

func (c *Client) SendVerifyEmail(u *model.User, emailAddress, magicLink string) error {
	plainText, html, err := c.tpl.RenderAdminEmail(&template.AdminEmail{
		Body:       c.tplStrVerifyEmail,
		ButtonText: "Verify",
		MagicLink:  magicLink,
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    _fromName,
		FromEmail:   _fromEmail,
		ToName:      u.FullName,
		ToEmail:     emailAddress,
		Subject:     "[convo] Verify Email",
		TextContent: plainText,
		HTMLContent: html,
	}

	return c.mail.Send(email)
}

//func (c *MailClient) SendMergeAccountsEmail(u *User, emailToMerge, magicLink string) error {
//	plainText, html, err := template.RenderAdminEmail(template.AdminEmail{
//		Body:       _tplStrMergeAccounts,
//		ButtonText: "Verify",
//		MagicLink:  magicLink,
//		Fargs:      []interface{}{emailToMerge, u.Email},
//	})
//	if err != nil {
//		return err
//	}

//	email := mail.EmailMessage{
//		FromName:    _fromName,
//		FromEmail:   _fromEmail,
//		ToName:      u.FullName,
//		ToEmail:     u.Email,
//		Subject:     "[convo] Verify Email",
//		TextContent: plainText,
//		HTMLContent: html,
//	}

//	return c.mail.Send(email)
//}

//func (c *MailClient) SendThread(thread *Thread, messages []*Message) error {
//	if len(messages) == 0 {
//		return errors.E(errors.Op("models.sendThread"), errors.Str("no messages to send"))
//	}

//	// From is the most recent message sender: messages[0].User.
//	sender, err := MapUserPartialToUser(messages[0].User, thread.Users)
//	if err != nil {
//		return err
//	}

//	// Loop through all participants and generate emails.
//	//
//	emailMessages := make([]mail.EmailMessage, len(thread.Users))
//	// Get the last five messages to be included in the email.
//	lastFive := getLastFive(messages)
//	for i, curUser := range thread.Users {
//		// Don't send an email to the sender.
//		if curUser.Key.Equal(sender.Key) {
//			continue
//		}

//		// Generate messages
//		tplMessages := make([]template.Message, len(lastFive))
//		for j, m := range lastFive {
//			tplMessages[j] = template.Message{
//				Body:     m.Body,
//				Name:     m.User.FirstName,
//				HasPhoto: m.HasPhoto(),
//				HasLink:  m.HasLink(),
//				FromID:   m.User.ID,
//				ToID:     curUser.ID,
//			}
//		}

//		plainText, html, err := template.RenderThread(template.Thread{
//			Subject:   thread.Subject,
//			FromName:  sender.FullName,
//			Messages:  tplMessages,
//			MagicLink: c.magic.NewLink(curUser.Key, curUser.Token, "magic"),
//		})
//		if err != nil {
//			return err
//		}

//		emailMessages[i] = mail.EmailMessage{
//			FromName:    sender.FullName,
//			FromEmail:   thread.GetEmail(),
//			ToName:      curUser.FullName,
//			ToEmail:     curUser.Email,
//			Subject:     thread.Subject,
//			TextContent: plainText,
//			HTMLContent: html,
//		}
//	}

//	for i := range emailMessages {
//		if emailMessages[i].FromEmail == "" {
//			continue
//		}

//		if err := c.mail.Send(emailMessages[i]); err != nil {
//			log.Alarm(errors.Errorf("models.sendThread: %v", err))
//		}
//	}

//	return nil
//}

//func (c *MailClient) SendEvent(event *Event, isUpdate bool) error {
//	var fmtStr string
//	if isUpdate {
//		fmtStr = "Updated invitation to %s"
//	} else {
//		fmtStr = "Invitation to %s"
//	}

//	// Loop through all participants and generate emails
//	emailMessages := make([]mail.EmailMessage, len(event.Users))
//	for i, curUser := range event.Users {
//		// Don't send invitations to the host
//		if event.OwnerIs(curUser) {
//			continue
//		}

//		plainText, html, err := template.RenderEvent(template.Event{
//			Name:        event.Name,
//			Address:     event.Address,
//			Time:        event.GetFormatedTime(),
//			Description: event.Description,
//			FromName:    event.Owner.FullName,
//			MagicLink: c.magic.NewLink(
//				curUser.Key,
//				strconv.FormatBool(!event.IsInFuture()),
//				fmt.Sprintf("rsvp/%s",
//					event.Key.Encode())),
//			ButtonText: "RSVP",
//		})
//		if err != nil {
//			return err
//		}

//		emailMessages[i] = mail.EmailMessage{
//			FromName:      event.Owner.FullName,
//			FromEmail:     event.GetEmail(),
//			ToName:        curUser.FullName,
//			ToEmail:       curUser.Email,
//			Subject:       fmt.Sprintf(fmtStr, event.Name),
//			TextContent:   plainText,
//			HTMLContent:   html,
//			ICSAttachment: event.GetICS(),
//		}
//	}

//	for i := range emailMessages {
//		if emailMessages[i].FromEmail == "" {
//			continue
//		}

//		if err := c.mail.Send(emailMessages[i]); err != nil {
//			log.Alarm(errors.Errorf("models.sendEvent: %v", err))
//		}
//	}

//	return nil
//}

//func (c *MailClient) SendEventInvitation(event *Event, user *User) error {
//	plainText, html, err := template.RenderEvent(template.Event{
//		Name:        event.Name,
//		Address:     event.Address,
//		Time:        event.GetFormatedTime(),
//		Description: event.Description,
//		FromName:    event.Owner.FullName,
//		MagicLink: c.magic.NewLink(
//			user.Key,
//			strconv.FormatBool(!event.IsInFuture()),
//			fmt.Sprintf("rsvp/%s",
//				event.Key.Encode())),
//		ButtonText: "RSVP",
//	})
//	if err != nil {
//		return err
//	}

//	email := mail.EmailMessage{
//		FromName:      event.Owner.FullName,
//		FromEmail:     event.GetEmail(),
//		ToName:        user.FullName,
//		ToEmail:       user.Email,
//		Subject:       fmt.Sprintf("Invitation to %s", event.Name),
//		TextContent:   plainText,
//		HTMLContent:   html,
//		ICSAttachment: event.GetICS(),
//	}

//	return c.mail.Send(email)
//}

//func (c *MailClient) SendCancellation(event *Event, message string) error {
//	// Loop through all participants and generate emails
//	emailMessages := make([]mail.EmailMessage, len(event.Users))
//	for i, curUser := range event.Users {
//		plainText, html, err := template.RenderCancellation(template.Event{
//			Name:     event.Name,
//			Address:  event.Address,
//			Time:     event.GetFormatedTime(),
//			FromName: event.Owner.FullName,
//			Message:  message,
//		})
//		if err != nil {
//			return err
//		}

//		emailMessages[i] = mail.EmailMessage{
//			FromName:    event.Owner.FullName,
//			FromEmail:   event.GetEmail(),
//			ToName:      curUser.FullName,
//			ToEmail:     curUser.Email,
//			Subject:     fmt.Sprintf("Cancelled: %s", event.Name),
//			TextContent: plainText,
//			HTMLContent: html,
//		}
//	}

//	for i := range emailMessages {
//		if emailMessages[i].FromEmail == "" {
//			continue
//		}

//		if err := c.mail.Send(emailMessages[i]); err != nil {
//			log.Alarm(errors.Errorf("models.sendCancellation: %v", err))
//		}
//	}

//	return nil
//}

//func (c *MailClient) SendDigest(digestList []DigestItem, upcomingEvents []*Event, user *User) error {
//	// Convert all the DigestItems into template.Threads with their messages
//	items := make([]template.Thread, len(digestList))
//	for i := range digestList {
//		messages := make([]template.Message, len(digestList[i].Messages))
//		for j := range messages {
//			messages[j] = template.Message{
//				Body:     digestList[i].Messages[j].Body,
//				Name:     digestList[i].Messages[j].User.FullName,
//				HasPhoto: digestList[i].Messages[j].HasPhoto(),
//				HasLink:  digestList[i].Messages[j].HasLink(),
//				FromID:   digestList[i].Messages[j].User.ID,
//				ToID:     user.ID,
//			}
//		}

//		items[i] = template.Thread{
//			Subject:  digestList[i].Name,
//			Messages: messages,
//		}
//	}

//	// Convert all of the upcomingEvents to template.Events
//	templateEvents := make([]template.Event, len(upcomingEvents))
//	for i := range upcomingEvents {
//		templateEvents[i] = template.Event{
//			Name:    upcomingEvents[i].Name,
//			Address: upcomingEvents[i].Address,
//			Time:    upcomingEvents[i].GetFormatedTime(),
//		}
//	}

//	// Render all the stuff
//	plainText, html, err := template.RenderDigest(template.Digest{
//		Items:     items,
//		Events:    templateEvents,
//		MagicLink: c.magic.NewLink(user.Key, user.Token, "magic"),
//	})
//	if err != nil {
//		return err
//	}

//	email := mail.EmailMessage{
//		FromName:    _fromName,
//		FromEmail:   _fromEmail,
//		ToName:      user.FullName,
//		ToEmail:     user.Email,
//		Subject:     "[convo] Digest",
//		TextContent: plainText,
//		HTMLContent: html,
//	}

//	return c.mail.Send(email)
//}

//func getLastFive(messages []*Message) []*Message {
//	if len(messages) > 5 {
//		return messages[:5]
//	}

//	return messages
//}

func readStringFromFile(file string) string {
	op := errors.Opf("mail.readStringFromFile(file=%s)", file)

	wd, err := os.Getwd()
	if err != nil {
		// This function should only be run at startup time, so we
		// panic if it fails.
		panic(errors.E(op, err))
	}

	var basePath string
	if strings.HasSuffix(wd, "mail") || strings.HasSuffix(wd, "integ") {
		// This package is the cwd, so we need to go up one dir to resolve the
		// layouts and includes dirs consistently.
		basePath = "../mail/content"
	} else {
		basePath = "./mail/content"
	}

	b, err := ioutil.ReadFile(path.Join(basePath, file))
	if err != nil {
		panic(err)
	}

	return string(b)
}
