package http

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("types tests", func() {
	It("should return true on a valid GUID", func() {
		Expect(isValidUUID("6e9a1e12-0721-47e8-b1b2-9a222a9a3080")).To(BeTrue())
	})
	It("should return false on an invalid GUID", func() {
		Expect(isValidUUID("NOT A VALID GUID")).To(BeFalse())
	})
	It("should return true on valid AllocateArgs", func() {
		Expect(validateAllocateArgs(&AllocateArgs{
			SessionID: "396022c2-caed-4bdf-98bb-521f2dc4f2f3",
			BuildID:   "b1b2d3e4-567f-4e4b-8f8b-f3a4b4a5b8e5",
		})).To(BeTrue())
	})
	It("should return false on invalid AllocateArgs", func() {
		Expect(validateAllocateArgs(&AllocateArgs{
			SessionID: "396022c2-caed-4bdf-98bb-521f2dc4f2f3",
			BuildID:   "WRONG",
		})).To(BeFalse())
	})
})
