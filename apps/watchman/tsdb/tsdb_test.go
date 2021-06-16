package tsdb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/config"
	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"

	"github.com/Pallinder/go-randomdata"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type BucketCleanup func() error

func CreateBucket(orgName, bucketName string) (string, BucketCleanup, error) {
	// u, err := client.UsersAPI().FindUserByName(context.Background(), "odyinflux")
	// if err != nil {
	// 	return "", nil, err
	// }
	// fmt.Println(client.OrganizationsAPI().GetOrganizations(context.Background()))
	orgObj, err := client.OrganizationsAPI().FindOrganizationByName(context.Background(), orgName)
	if err != nil {
		return "", nil, err
	}
	// permOrgs := &domain.Permission{
	// 	Action: domain.PermissionActionRead,
	// 	Resource: domain.Resource{
	// 		Type: domain.ResourceTypeOrgs,
	// 	},
	// }

	// permissionWrite := &domain.Permission{
	// 	Action: domain.PermissionActionWrite,
	// 	Resource: domain.Resource{
	// 		Type: domain.ResourceTypeBuckets,
	// 	},
	// }

	// // create read permission for buckets
	// permissionRead := &domain.Permission{
	// 	Action: domain.PermissionActionRead,
	// 	Resource: domain.Resource{
	// 		Type: domain.ResourceTypeBuckets,
	// 	},
	// }
	// permissions := []domain.Permission{*permOrgs}
	// auth := &domain.Authorization{
	// 	OrgID:       org.Id,
	// 	Permissions: &permissions,
	// 	User:        &user.Name,
	// 	UserID:      user.Id,
	// }
	// oid := "53d7fcc3c19b430d"
	b, err := client.BucketsAPI().CreateBucket(
		context.Background(),
		&domain.Bucket{Name: bucketName, OrgID: orgObj.Id, RetentionRules: domain.RetentionRules{domain.RetentionRule{EverySeconds: 0}}})
	if err != nil {
		return "", nil, err
	}
	return b.Name, func() error { return client.BucketsAPI().DeleteBucket(context.Background(), b) }, nil
}

func TestWrite(t *testing.T) {
	cfg, err := config.Read()
	require.NoError(t, err)
	ifCfg := cfg.GetStringMapString("influxdb")
	Connect(ifCfg["url"], ifCfg["token"])

	b, cleanup, err := CreateBucket(ifCfg["org"], randomdata.Alphanumeric(16))
	require.NoError(t, err)
	defer cleanup()
	ConfigBucket(ifCfg["org"], b)

	rep := playbackReportFactory.MustCreate().(*reporter.PlaybackReport)
	Write(rep, randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()))

	recMap := map[string]int64{}
	timeout := time.After(time.Second * 10)
	for len(recMap) == 0 {
		select {
		case <-timeout:
			break
		default:
			res, err := qapi.Query(context.Background(), fmt.Sprintf(`from(bucket:"%v")|> range(start: -14d)`, b))
			require.NoError(t, err)

			for res.Next() {
				require.NoError(t, res.Err())
				rec := res.Record()
				recMap[rec.Field()] = rec.Value().(int64)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t,
		map[string]int64{
			"rebuf_count":    int64(rep.RebufCount),
			"rebuf_duration": int64(rep.RebufDuration),
			"client_rate":    int64(*rep.ClientRate),
		},
		recMap,
	)

	err = client.DeleteAPI().DeleteWithName(context.Background(), org, bucket, time.Now().Add(-15*time.Second), time.Now(), "")
	assert.NoError(t, err)
}
