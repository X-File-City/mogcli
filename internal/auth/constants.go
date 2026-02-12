package auth

var DefaultDelegatedScopes = []string{
	"openid",
	"profile",
	"offline_access",
	"User.Read",
	"Mail.Read",
	"Mail.Send",
	"Calendars.Read",
	"Calendars.ReadWrite",
	"Contacts.Read",
	"Contacts.ReadWrite",
	"Tasks.Read",
	"Tasks.ReadWrite",
	"Files.Read",
	"Files.ReadWrite",
	"Group.Read.All",
	"GroupMember.Read.All",
}
