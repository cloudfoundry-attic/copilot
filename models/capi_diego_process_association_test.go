package models_test

import (
	"code.cloudfoundry.org/copilot/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CAPIDiegoProcessAssociationsRepo", func() {
	var capiDiegoProcessAssociationsRepo models.CAPIDiegoProcessAssociationsRepo
	BeforeEach(func() {
		capiDiegoProcessAssociationsRepo = models.CAPIDiegoProcessAssociationsRepo{
			Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
		}
	})

	It("can upsert and delete CAPIDiegoProcessAssociations", func() {
		capiDiegoProcessAssociation := models.CAPIDiegoProcessAssociation{
			CAPIProcessGUID: "some-capi-process-guid",
			DiegoProcessGUIDs: models.DiegoProcessGUIDs{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			},
		}

		go capiDiegoProcessAssociationsRepo.Upsert(&capiDiegoProcessAssociation)

		capiProcessGUID := models.CAPIProcessGUID("some-capi-process-guid")

		Eventually(func() *models.CAPIDiegoProcessAssociation {
			return capiDiegoProcessAssociationsRepo.Get(&capiProcessGUID)
		}).Should(Equal(&capiDiegoProcessAssociation))

		capiDiegoProcessAssociationsRepo.Delete(&capiDiegoProcessAssociation.CAPIProcessGUID)
		Expect(capiDiegoProcessAssociationsRepo.Get(&capiProcessGUID)).To(BeNil())
	})

	It("can sync CAPIDiegoProcessAssociations", func() {
		capiDiegoProcessAssociation := &models.CAPIDiegoProcessAssociation{
			CAPIProcessGUID: "some-capi-process-guid",
			DiegoProcessGUIDs: models.DiegoProcessGUIDs{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			},
		}

		capiDiegoProcessAssociationsRepo.Upsert(capiDiegoProcessAssociation)

		newCapiDiegoProcessAssociation := &models.CAPIDiegoProcessAssociation{
			CAPIProcessGUID: "some-other-capi-process-guid",
			DiegoProcessGUIDs: models.DiegoProcessGUIDs{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			},
		}

		capiDiegoProcessAssociationsRepo.Sync([]*models.CAPIDiegoProcessAssociation{newCapiDiegoProcessAssociation})

		Expect(capiDiegoProcessAssociationsRepo.List()).To(Equal(map[models.CAPIProcessGUID]*models.DiegoProcessGUIDs{
			"some-other-capi-process-guid": &newCapiDiegoProcessAssociation.DiegoProcessGUIDs,
		}))
	})
})
