package media

import (
	"context"
	"fmt"
)

type byPathType map[PathType]PathResolver

var resolverTable = byPathType{
	PathTypeFileVideo: &fileVideoResolver{},
	PathTypeFileISO:   &fileISOResolver{},
	PathTypeDirBDMV:   &dirBDMVResolver{},
	PathTypeDirDVD:    &dirDVDResolver{},
	PathTypeDirISO:    &dirISOResolver{},
	PathTypeDirVideo:  &dirVideoResolver{},
}

func resolveResolver(pt PathType) PathResolver {
	if r, ok := resolverTable[pt]; ok {
		return r
	}
	return nil
}

func contextBack(ctx ResolveContext) context.Context {
	if ctx.Context != nil {
		return ctx.Context
	}
	return context.Background()
}

type fileVideoResolver struct{}

func (r *fileVideoResolver) ResolveScreenshot(ctx ResolveContext) (string, func(), error) {
	return ctx.Input, func() {}, nil
}

func (r *fileVideoResolver) ResolveBDInfo(_ ResolveContext) (string, func(), error) {
	return "", func() {}, errNotBDMV
}

func (r *fileVideoResolver) ResolveMediaInfo(ctx ResolveContext, _ int) ([]string, func(), error) {
	return []string{ctx.Input}, func() {}, nil
}

type fileISOResolver struct{}

func (r *fileISOResolver) ResolveScreenshot(ctx ResolveContext) (string, func(), error) {
	return resolveM2TSFromMountedISO(contextBack(ctx), ctx.Input)
}

func (r *fileISOResolver) ResolveBDInfo(ctx ResolveContext) (string, func(), error) {
	return resolveBDInfoFromMountedISO(contextBack(ctx), ctx.Input)
}

func (r *fileISOResolver) ResolveMediaInfo(ctx ResolveContext, _ int) ([]string, func(), error) {
	return []string{ctx.Input}, func() {}, nil
}

type dirBDMVResolver struct{}

func (r *dirBDMVResolver) ResolveScreenshot(ctx ResolveContext) (string, func(), error) {
	bdmvRoot, ok := resolveBDMVRoot(ctx.Input)
	if !ok {
		bdmvRoot = ctx.Input
	}
	m2ts, err := findLargestM2TS(bdmvRoot)
	if err != nil {
		return "", func() {}, err
	}
	return m2ts, func() {}, nil
}

func (r *dirBDMVResolver) ResolveBDInfo(ctx ResolveContext) (string, func(), error) {
	bdRoot, ok := resolveBDInfoRoot(ctx.Input)
	if !ok {
		return "", func() {}, errNotBDMV
	}
	return bdRoot, func() {}, nil
}

func (r *dirBDMVResolver) ResolveMediaInfo(ctx ResolveContext, _ int) ([]string, func(), error) {
	m2ts, cleanup, err := r.ResolveScreenshot(ctx)
	if err != nil {
		return nil, func() {}, err
	}
	return []string{m2ts}, cleanup, nil
}

type dirDVDResolver struct{}

func (r *dirDVDResolver) ResolveScreenshot(_ ResolveContext) (string, func(), error) {
	return "", func() {}, errNotSupported
}

func (r *dirDVDResolver) ResolveBDInfo(_ ResolveContext) (string, func(), error) {
	return "", func() {}, errNotBDMV
}

func (r *dirDVDResolver) ResolveMediaInfo(_ ResolveContext, _ int) ([]string, func(), error) {
	return nil, func() {}, errNotSupported
}

type dirISOResolver struct{}

func (r *dirISOResolver) ResolveScreenshot(ctx ResolveContext) (string, func(), error) {
	isoPath, err := findISOInDir(ctx.Input)
	if err != nil {
		return "", func() {}, err
	}
	return resolveM2TSFromMountedISO(contextBack(ctx), isoPath)
}

func (r *dirISOResolver) ResolveBDInfo(ctx ResolveContext) (string, func(), error) {
	isoPath, err := findISOInDir(ctx.Input)
	if err != nil {
		return "", func() {}, err
	}
	return resolveBDInfoFromMountedISO(contextBack(ctx), isoPath)
}

func (r *dirISOResolver) ResolveMediaInfo(ctx ResolveContext, _ int) ([]string, func(), error) {
	isoPath, err := findISOInDir(ctx.Input)
	if err != nil {
		return nil, func() {}, err
	}
	return []string{isoPath}, func() {}, nil
}

type dirVideoResolver struct{}

func (r *dirVideoResolver) ResolveScreenshot(ctx ResolveContext) (string, func(), error) {
	videoPath := findVideoFile(ctx.Input)
	if videoPath == "" {
		return "", func() {}, fmt.Errorf("no video file found in %s", ctx.Input)
	}
	return videoPath, func() {}, nil
}

func (r *dirVideoResolver) ResolveBDInfo(ctx ResolveContext) (string, func(), error) {
	bdmvPath := findBDMVInSubdirs(ctx.Input)
	if bdmvPath != "" {
		if bdRoot, ok := resolveBDInfoRoot(bdmvPath); ok {
			return bdRoot, func() {}, nil
		}
	}
	isoPath := findISOInSubdirs(ctx.Input)
	if isoPath != "" {
		return resolveBDInfoFromMountedISO(contextBack(ctx), isoPath)
	}
	return "", func() {}, errNotBDMV
}

func (r *dirVideoResolver) ResolveMediaInfo(ctx ResolveContext, limit int) ([]string, func(), error) {
	candidates, err := findVideoCandidates(ctx.Input, limit)
	if err != nil {
		return nil, func() {}, err
	}
	return candidates, func() {}, nil
}

var errNotBDMV = &resolveError{"path does not contain BDMV or BDISO content"}
var errNotSupported = &resolveError{"path type not supported for this operation"}

type resolveError struct{ msg string }

func (e *resolveError) Error() string { return e.msg }