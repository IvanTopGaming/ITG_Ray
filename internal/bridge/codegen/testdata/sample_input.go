// Package testdata is read by the codegen test as a self-contained schema.
package testdata

type Empty struct{}

type SampleService interface {
	DoThing(p SampleParams) (SampleResult, error)
}

type SampleParams struct {
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

type SampleResult struct {
	OK bool `json:"ok"`
}

type EventTopic string

const TopicSampleEvent EventTopic = "sample.event"
