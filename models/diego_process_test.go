package models_test

import (
	"code.cloudfoundry.org/copilot/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiegoProcessGUIDs", func() {
	Describe("ToStringSlice", func() {
		It("returns the guids as a slice of strings", func() {
			diegoProcessGUIDs := models.DiegoProcessGUIDs{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			}
			Expect(diegoProcessGUIDs.ToStringSlice()).To(Equal([]string{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			}))
		})
	})
})

var _ = Describe("DiegoProcessGUIDsFromStringSlice", func() {
	It("returns the guids as DiegoProcessGUIDs", func() {
		diegoProcessGUIDs := []string{
			"some-diego-process-guid-1",
			"some-diego-process-guid-2",
		}
		Expect(models.DiegoProcessGUIDsFromStringSlice(diegoProcessGUIDs)).To(Equal(models.DiegoProcessGUIDs{
			"some-diego-process-guid-1",
			"some-diego-process-guid-2",
		}))
	})
})
