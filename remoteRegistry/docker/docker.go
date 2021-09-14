package docker

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pubg/kube-image-deployer/interfaces"
	"github.com/pubg/kube-image-deployer/util"
)

type RemoteRegistryDocker struct {
	imageAuthMap map[string]authn.Keychain
	cache        *util.Cache
}

// NewRemoteRegistry returns a new RemoteRegistryDocker
func NewRemoteRegistry(imageAuthMap map[string]authn.Keychain, cacheTTL uint) interfaces.IRemoteRegistry {
	d := &RemoteRegistryDocker{
		imageAuthMap: imageAuthMap,
		cache:        util.NewCache(cacheTTL),
	}

	return d
}

// GetImage returns a docker image digest hash from url:tag
func (d *RemoteRegistryDocker) GetImageString(url, tag string) (string, error) {
	if strings.Contains(tag, "*") {
		// *을 포함하는 경우 전체 tag에서 가장 높은 tag를 찾아 반환한다.
		return d.getImageHighestVersionTag(url, tag)
	} else {
		// 단일 tag인 경우 가장 최신 sha256 digest를 반환한다.
		return d.getImageDigestHash(url, tag)
	}
}

func (d *RemoteRegistryDocker) getImageDigestHash(url, tag string) (string, error) {

	fullUrl := fmt.Sprintf("%s:%s", url, tag)
	authKeyChain := d.getAuthKeyChain(url)
	ref, err := name.ParseReference(fullUrl)

	if err != nil {
		return "", err
	}

	hash, err := d.cache.Get(fullUrl, func() (interface{}, error) {
		if img, err := remote.Image(ref, remote.WithAuthFromKeychain(authKeyChain)); err == nil {
			if digest, err := img.Digest(); err == nil {
				return digest.String(), nil
			} else {
				return "", err
			}
		} else {
			return "", err
		}
	})

	return url + "@" + hash.(string), err

}

func (d *RemoteRegistryDocker) getAuthKeyChain(url string) authn.Keychain {

	for key, value := range d.imageAuthMap {
		if strings.HasPrefix(url, key) {
			return value
		}
	}

	return authn.DefaultKeychain

}

func (d *RemoteRegistryDocker) getImageHighestVersionTag(url, tag string) (string, error) {
	authKeyChain := d.getAuthKeyChain(url)
	repo, err := name.NewRepository(url)
	if nil != err {
		return "", err
	}

	cacheKey := url + "___" + tag
	target, err := d.cache.Get(cacheKey, func() (interface{}, error) {
		tags, err := remote.List(repo, remote.WithAuthFromKeychain(authKeyChain))
		if nil != err {
			return "", err
		}

		t, err := util.GetHighestVersionWithFilter(tags, tag)
		if nil != err {
			return "", err
		}

		return t, nil
	})
	if nil != err {
		return "", err
	}

	fullUrl := fmt.Sprintf("%s:%s", url, target.(string))

	return fullUrl, nil
}
