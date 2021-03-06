package gae

import (
	"appengine"
	"appengine/blobstore"
	"appengine/file"
	"code.google.com/p/go.net/context"
	"encoding/json"
	"fmt"
	"github.com/golang/oauth2/google"
	"github.com/snakehopper/gcloud-golang/storage"
	"google.golang.org/cloud"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"
)

type GStorage struct {
	c   appengine.Context
	ctx context.Context
	// bucket is the Google Cloud Storage bucket name used for the GStorage.
	bucket string
	// failed indicates that one or more of the GStorage steps failed.
	failed bool
}

func NewGStorage(c appengine.Context) (*GStorage, error) {
	bucketName, err := file.DefaultBucketName(c)
	if err != nil {
		c.Errorf("failed to get default GCS bucket name: %v", err)
		return nil, err
	}

	config := google.NewAppEngineConfig(c, []string{storage.ScopeFullControl})
	ctx := cloud.NewContext(appengine.AppID(c), &http.Client{Transport: config.NewTransport()})

	gs := &GStorage{
		c:      c,
		ctx:    ctx,
		bucket: bucketName,
	}
	return gs, nil
}

func (gs *GStorage) CreateImageFile(fileName string, img multipart.File, ct string) error {
	wc := storage.NewWriter(gs.ctx, gs.bucket, fileName, &storage.Object{
		ContentType: ct,
		//Metadata:    map[string]string{},
	})

	b, err := ioutil.ReadAll(img)
	if err != nil {
		return err
	}

	if _, err := wc.Write(b); err != nil {
		return fmt.Errorf("createFile: unable to write data to bucket %q, file %q: %v", gs.bucket, fileName, err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("createFile: unable to close bucket %q, file %q: %v", gs.bucket, fileName, err)
	}
	// Wait for the file to be fully written.
	_, err = wc.Object()
	if err != nil {
		return fmt.Errorf("createFile: unable to finalize file from bucket %q, file %q: %v", gs.bucket, fileName, err)
	}

	return nil
}

// CreateFile creates a file in Google Cloud Storage.
func (gs *GStorage) CreateJsonFile(fileName string, v interface{}) error {
	wc := storage.NewWriter(gs.ctx, gs.bucket, fileName, &storage.Object{
		ContentType:  "application/json",
		CacheControl: "private, max-age=0, no-transform",
		Metadata:     map[string]string{},
	})

	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("putVendorStaticFile failed, due to json.Marshal", err)
	}

	if _, err := wc.Write(b); err != nil {
		return fmt.Errorf("createFile: unable to write data to bucket %q, file %q: %v", gs.bucket, fileName, err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("createFile: unable to close bucket %q, file %q: %v", gs.bucket, fileName, err)
	}
	// Wait for the file to be fully written.
	_, err = wc.Object()
	if err != nil {
		return fmt.Errorf("createFile: unable to finalize file from bucket %q, file %q: %v", gs.bucket, fileName, err)
	}

	return nil
}

func (gs *GStorage) CopyBlob(src appengine.BlobKey, v ImageBlober) (appengine.BlobKey, error) {
	sObj, err := gs.ReadBlobKey(src)
	if err != nil {
		return "", err
	}

	var srcName = sObj.Name
	sObj.Name = v.BucketPath()
	_, err = storage.CopyObject(gs.ctx, gs.bucket, srcName, sObj)

	gcsFilename := "/" + strings.Join([]string{"gs", sObj.Bucket, sObj.Name}, "/")
	return blobstore.BlobKeyForFile(gs.c, gcsFilename)
}

func (gs *GStorage) ReadBlobKey(src appengine.BlobKey) (*storage.Object, error) {
	info, err := blobstore.Stat(gs.c, src)
	if err != nil {
		return nil, err
	}

	bucket := appengine.DefaultVersionHostname(gs.c)
	name := strings.TrimPrefix(info.ObjectName, "/"+bucket+"/")

	return storage.StatObject(gs.ctx, bucket, name)
}
