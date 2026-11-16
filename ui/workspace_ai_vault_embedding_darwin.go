package ui

import (
	"context"
	"errors"
	"fmt"
	"structs"
	"sync"

	"cyberwriter/internal/core/vaultcontext"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var openNaturalLanguage = sync.OnceValue(func() error {
	_, err := purego.Dlopen("/System/Library/Frameworks/NaturalLanguage.framework/NaturalLanguage",
		purego.RTLD_LAZY|purego.RTLD_LOCAL)
	return err
})

type vaultEmbeddingRange struct {
	_                structs.HostLayout
	Location, Length uint
}

type vaultContextualModel struct {
	value, language objc.ID
	id              string
	dimension       uint
}

func newVaultContextualModel() (vaultContextualModel, error) {
	if err := openNaturalLanguage(); err != nil {
		return vaultContextualModel{}, err
	}
	// EV-UI-292. The native workers use the English contextual model.
	language := nativeNSString("en")
	embedding := objc.ID(objc.GetClass("NLContextualEmbedding"))
	value := embedding.Send(objc.RegisterName("contextualEmbeddingWithLanguage:"), language)
	if value == 0 {
		return vaultContextualModel{}, errVaultContextualUnsupported
	}
	return vaultContextualModel{value: value, language: language}, nil
}

// The caller holds an autorelease pool for the life of this model.
func openVaultContextualModel() (vaultContextualModel, error) {
	model, err := newVaultContextualModel()
	if err != nil {
		return vaultContextualModel{}, err
	}
	if !objc.Send[bool](model.value, objc.RegisterName("hasAvailableAssets")) {
		return vaultContextualModel{}, errVaultContextualAssetsMissing
	}
	if err := model.load(); err != nil {
		return vaultContextualModel{}, err
	}
	model.id = objc.Send[string](model.value.Send(objc.RegisterName("modelIdentifier")), objc.RegisterName("UTF8String"))
	model.dimension = objc.Send[uint](model.value, objc.RegisterName("dimension"))
	return model, nil
}

func (model vaultContextualModel) load() error {
	var problem objc.ID
	if !objc.Send[bool](model.value, objc.RegisterName("loadWithError:"), &problem) {
		return vaultContextualError(problem, "Apple contextual embedding could not load")
	}
	return nil
}

func vaultContextualError(problem objc.ID, caption string) error {
	if problem != 0 {
		message := objc.Send[string](problem.Send(objc.RegisterName("localizedDescription")), objc.RegisterName("UTF8String"))
		if message != "" {
			return fmt.Errorf("%s. %s", caption, message)
		}
	}
	return errors.New(caption)
}

func (model vaultContextualModel) close() {
	model.value.Send(objc.RegisterName("unload"))
}

func requestVaultContextualAssets(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done, err := beginVaultContextualAssets()
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func beginVaultContextualAssets() (<-chan error, error) {
	defer nativeAutoreleasePool()()
	model, err := newVaultContextualModel()
	if err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	if objc.Send[bool](model.value, objc.RegisterName("hasAvailableAssets")) {
		done <- nil
		return done, nil
	}
	// The system retains its completion block. Keep the model alive until
	// that callback, even if the originating Vault has already closed.
	model.value.Send(objc.RegisterName("retain"))
	completion := objc.NewBlock(func(_ objc.Block, outcome int, problem objc.ID) {
		defer nativeAutoreleasePool()()
		defer model.value.Send(objc.RegisterName("release"))
		const available = 0
		if outcome != available || problem != 0 {
			done <- errors.New("Couldn't download embedding model. Open System Settings → Apple Intelligence and try again.")
			return
		}
		if err := model.load(); err != nil {
			done <- err
			return
		}
		model.close()
		done <- nil
	})
	defer completion.Release()
	model.value.Send(objc.RegisterName("requestEmbeddingAssetsWithCompletionHandler:"), completion)
	return done, nil
}

func (model vaultContextualModel) embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer nativeAutoreleasePool()()
	const utf8 = uint(4)
	// Pass the byte count to NSString so embedded NUL is preserved.
	source := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("alloc")).
		Send(objc.RegisterName("initWithBytes:length:encoding:"), text, uint(len(text)), utf8)
	if source == 0 {
		return nil, errors.New("the Vault text could not be decoded as UTF-8")
	}
	defer source.Send(objc.RegisterName("release"))
	var problem objc.ID
	result := model.value.Send(objc.RegisterName("embeddingResultForString:language:error:"), source, model.language, &problem)
	if result == 0 {
		return nil, vaultContextualError(problem, "Apple contextual embedding could not read the text")
	}
	vector := make([]float32, model.dimension)
	tokens := 0
	count := objc.RegisterName("count")
	at := objc.RegisterName("objectAtIndex:")
	double := objc.RegisterName("doubleValue")
	block := objc.NewBlock(func(_ objc.Block, token objc.ID, _ vaultEmbeddingRange, stop *bool) {
		if ctx.Err() != nil {
			*stop = true
			return
		}
		for column := range min(model.dimension, objc.Send[uint](token, count)) {
			vector[column] += float32(objc.Send[float64](token.Send(at, column), double))
		}
		tokens++
	})
	defer block.Release()
	size := objc.Send[uint](source, objc.RegisterName("length"))
	result.Send(objc.RegisterName("enumerateTokenVectorsInRange:usingBlock:"), vaultEmbeddingRange{Length: size}, block)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tokens == 0 {
		return nil, errors.New("Apple contextual embedding returned no tokens")
	}
	scale := float32(1) / float32(tokens)
	for column := range vector {
		vector[column] *= scale
	}
	return vector, nil
}

func buildContextualVaultIndex(ctx context.Context, snapshot aiVaultBuildSnapshot) (*vaultcontext.Index, []vaultcontext.MapSkippedFile, error) {
	defer nativeAutoreleasePool()()
	model, err := openVaultContextualModel()
	if err != nil {
		return nil, nil, err
	}
	defer model.close()
	return snapshot.buildIndex(ctx, model.id, int(model.dimension), model.embed)
}

func searchContextualVaultIndex(ctx context.Context, index *vaultcontext.Index, prompt string) ([]vaultcontext.Match, error) {
	defer nativeAutoreleasePool()()
	model, err := openVaultContextualModel()
	if err != nil {
		return nil, err
	}
	defer model.close()
	if model.id != index.ModelID() {
		return nil, errors.New("the Apple embedding model has changed. Rebuild the Vault index in AI Settings")
	}
	if index.ChunkCount() == 0 {
		return nil, nil
	}
	query, err := model.embed(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return index.SearchVector(ctx, query)
}
