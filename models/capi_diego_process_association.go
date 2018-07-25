package models

import "sync"

type CAPIDiegoProcessAssociationsRepo struct {
	Repo map[CAPIProcessGUID]*CAPIDiegoProcessAssociation
	sync.Mutex
}

type CAPIDiegoProcessAssociation struct {
	CAPIProcessGUID   CAPIProcessGUID
	DiegoProcessGUIDs DiegoProcessGUIDs
}

func (c *CAPIDiegoProcessAssociationsRepo) Upsert(capiDiegoProcessAssociation *CAPIDiegoProcessAssociation) {
	c.Lock()
	c.Repo[capiDiegoProcessAssociation.CAPIProcessGUID] = capiDiegoProcessAssociation
	c.Unlock()
}

func (c *CAPIDiegoProcessAssociationsRepo) Delete(capiProcessGUID *CAPIProcessGUID) {
	c.Lock()
	delete(c.Repo, *capiProcessGUID)
	c.Unlock()
}

func (c *CAPIDiegoProcessAssociationsRepo) Sync(capiDiegoProcessAssociations []*CAPIDiegoProcessAssociation) {
	repo := make(map[CAPIProcessGUID]*CAPIDiegoProcessAssociation)
	for _, capiDiegoProcessAssociation := range capiDiegoProcessAssociations {
		repo[capiDiegoProcessAssociation.CAPIProcessGUID] = capiDiegoProcessAssociation
	}
	c.Lock()
	c.Repo = repo
	c.Unlock()
}

func (c *CAPIDiegoProcessAssociationsRepo) List() map[CAPIProcessGUID]*DiegoProcessGUIDs {
	list := make(map[CAPIProcessGUID]*DiegoProcessGUIDs)

	c.Lock()
	for k, v := range c.Repo {
		list[k] = &v.DiegoProcessGUIDs
	}
	c.Unlock()

	return list
}

func (c *CAPIDiegoProcessAssociationsRepo) Get(capiProcessGUID *CAPIProcessGUID) *CAPIDiegoProcessAssociation {
	c.Lock()
	capiDiegoProcessAssociation, _ := c.Repo[*capiProcessGUID]
	c.Unlock()
	return capiDiegoProcessAssociation
}
