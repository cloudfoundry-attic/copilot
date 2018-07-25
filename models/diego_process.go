package models

const CF_APP_SSH_PORT = 2222

type DiegoProcessGUID string

type DiegoProcessGUIDs []DiegoProcessGUID

func DiegoProcessGUIDsFromStringSlice(diegoProcessGUIDs []string) DiegoProcessGUIDs {
	diegoGUIDs := DiegoProcessGUIDs{}
	for _, diegoGUID := range diegoProcessGUIDs {
		diegoGUIDs = append(diegoGUIDs, DiegoProcessGUID(diegoGUID))
	}
	return diegoGUIDs
}

func (p DiegoProcessGUIDs) ToStringSlice() []string {
	diegoGUIDs := []string{}
	for _, diegoGUID := range p {
		diegoGUIDs = append(diegoGUIDs, string(diegoGUID))
	}
	return diegoGUIDs
}
