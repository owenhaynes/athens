package subdirfetcher

import (
	"context"

 "bytes"

	"strings"
"io/ioutil"
	"encoding/json"
	"github.com/gomods/athens/pkg/errors"
	"github.com/gomods/athens/pkg/module"
	"github.com/gomods/athens/pkg/module/subdirfetcher/gosrc/modfetch/codehost"
	"github.com/gomods/athens/pkg/module/subdirfetcher/gosrc/modfetch"
	"github.com/gomods/athens/pkg/storage"
	"github.com/spf13/afero"
)


type VanityURL struct {
	Prefix string
	Git string
	Offset string
}

type VanityURLS []VanityURL

func (v *VanityURLS) FindPrefix(mod string) (VanityURL, bool) {
	for _, vv := range *v {
		if strings.HasPrefix(mod, vv.Prefix) {
			return vv, true
		}
	}
	return VanityURL{}, false
}

var vanityURLS = VanityURLS{
	VanityURL{
		Prefix: "foo.com/go/pkg/test",
		Git: "git@bitbucket.org/fooOrg/test",
		Offset: "/src/golang",
	},
}


// NewSubdirFetcher ...
func NewSubdirFetcher(goBinaryName string, goProxy string, gfs afero.Fs) (module.Fetcher, error) {
	goGetFetcher, err := module.NewGoGetFetcher(goBinaryName, goProxy, gfs)
	if err != nil {
		return nil, err
	}
	return &Fetcher{
		goGet: goGetFetcher,
		fs:    gfs,
	}, nil
}

// Fetcher ...
type Fetcher struct {
	goGet module.Fetcher
	fs    afero.Fs
}

// Fetch downloads the sources from an upstream and returns the corresponding
// .info, .mod, and .zip files.
func (f *Fetcher) Fetch(ctx context.Context, mod, ver string) (*storage.Version, error) {
	const op errors.Op = "SubdirFetcher.Fetch"

	vanity, ok := vanityURLS.FindPrefix(mod)

	if !ok {
		return f.goGet.Fetch(ctx, mod, ver)
	}

	goPathRoot, err := afero.TempDir(f.fs, "", "athens")
	if err != nil {
		return nil, errors.E(op, err)
	}

	//src/golang

	codeHost, err := codehost.NewGitRepo(goPathRoot, vanity.Git, false, vanity.Offset)

	if err != nil {
		return nil, errors.E(op, err)
	}

	codeRepo, err := modfetch.NewCodeRepo(codeHost, mod, mod, vanity.Offset)

	if err != nil {
		return nil, errors.E(op, err)
	}

	revInfo, err := codeHost.Stat(ver)
	if err != nil {
		return nil,  errors.E(op, err)
	}

	info, err := codeRepo.(*modfetch.CodeRepo).Convert(revInfo, ver)
	if err != nil {
		return nil,  errors.E(op, err)
	}

	gomod, err := codeRepo.GoMod(info.Version)
	if err != nil {
		return nil,  errors.E(op, err)
	}

	var bb bytes.Buffer
	if err := codeRepo.Zip(&bb, info.Version); err != nil {
		return nil,  errors.E(op, err)
	}

	infoJSON, err := json.Marshal(&info)
	if err != nil {
		return nil,  errors.E(op, err)
	}

	return &storage.Version{
		Mod: gomod,
		Zip: ioutil.NopCloser(&bb),
		Info: infoJSON,
		Semver: info.Version,
	}, nil
}



