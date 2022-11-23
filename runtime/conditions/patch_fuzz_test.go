package conditions

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Fuzz_PatchApply(f *testing.F) {
	f.Fuzz(func(t *testing.T,
		beforeData, afterData, setterData []byte) {

		before, err := newFake(fuzz.NewConsumer(beforeData))
		if err != nil {
			return
		}

		after, err := newFake(fuzz.NewConsumer(afterData))
		if err != nil {
			return
		}

		patch := NewPatch(before, after)
		setter, err := newFake(fuzz.NewConsumer(setterData))
		if err != nil {
			return
		}

		_ = patch.Apply(setter)
	})
}

func newFake(fc *fuzz.ConsumeFuzzer) (*testdata.Fake, error) {
	obj := &testdata.Fake{}
	noOfConditions, err := fc.GetInt()
	if err != nil {
		return obj, err
	}

	maxNoOfConditions := 30
	conditions := make([]metav1.Condition, 0)
	for i := 0; i < noOfConditions%maxNoOfConditions; i++ {
		c := metav1.Condition{}
		err = fc.GenerateStruct(&c)
		if err != nil {
			return obj, err
		}

		conditions = append(conditions, c)
	}
	obj.SetConditions(conditions)
	return obj, nil
}
