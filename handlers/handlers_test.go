package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/redscaresu/mockway/testutil"
)

func TestAuthRequiredOnScalewayRoutes(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/vpc/v1/regions/fr-par/vpcs", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestInstanceProductsServersCatalog(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/products/servers")
	require.Equal(t, 200, status)

	servers := body["servers"].(map[string]any)
	for _, typ := range []string{"DEV1-S", "DEV1-M", "DEV1-L", "GP1-XS", "GP1-S", "GP1-M", "GP1-L", "GP1-XL"} {
		v, ok := servers[typ]
		require.True(t, ok, "missing server type %s", typ)
		entry := v.(map[string]any)
		require.Contains(t, entry, "monthly_price")
		require.Contains(t, entry, "hourly_price")
		require.Contains(t, entry, "ncpus")
		require.Contains(t, entry, "ram")
		require.Contains(t, entry, "arch")
		require.Contains(t, entry, "volumes_constraint")
		volumes := entry["volumes_constraint"].(map[string]any)
		require.Contains(t, volumes, "min_size")
		require.Contains(t, volumes, "max_size")
		require.Contains(t, entry, "per_volume_constraint")
		perVolume := entry["per_volume_constraint"].(map[string]any)
		lssd := perVolume["l_ssd"].(map[string]any)
		require.Contains(t, lssd, "min_size")
		require.Contains(t, lssd, "max_size")
		require.Equal(t, "l_ssd", entry["volume_type"])
		require.Equal(t, "l_ssd", entry["default_volume_type"])
	}

	status, paged := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/products/servers?page=1")
	require.Equal(t, 200, status)
	require.Equal(t, servers, paged["servers"])
}

func TestUnimplementedUnknownPathReturns501(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/does-not-exist")
	require.Equal(t, 501, status)
	require.Equal(t, "not_implemented", body["type"])
	require.Contains(t, body["message"], "GET /instance/v1/zones/fr-par-1/does-not-exist")
}

func TestUnimplementedMethodReturns501(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := doPut(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "x"})
	require.Equal(t, 501, status)
	require.Equal(t, "not_implemented", body["type"])
	require.Contains(t, body["message"], "PUT /instance/v1/zones/fr-par-1/servers")
}

func TestMarketplaceLocalImagesListFilter(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=ubuntu_noble&zone=fr-par-1&type=instance_sbs")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), body["total_count"])
	images := body["local_images"].([]any)
	require.Len(t, images, 1)
	img := images[0].(map[string]any)
	require.Equal(t, "ubuntu_noble", img["label"])
	require.Equal(t, "fr-par-1", img["zone"])
	require.Equal(t, "instance_sbs", img["type"])
}

func TestMarketplaceLocalImagesCompatibleTypesContainsDEV1S(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=ubuntu_noble&zone=fr-par-1&type=instance_sbs")
	require.Equal(t, 200, status)
	images := body["local_images"].([]any)
	require.NotEmpty(t, images)
	img := images[0].(map[string]any)
	compatible := img["compatible_commercial_types"].([]any)
	require.Contains(t, compatible, "DEV1-S")
}

func TestMarketplaceLocalImagesUnknownLabelEmpty(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=not_real&zone=fr-par-1&type=instance_sbs")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])
	require.Len(t, body["local_images"].([]any), 0)
}

func TestMarketplaceLocalImageGetByID(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, list := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=ubuntu_noble&zone=fr-par-1&type=instance_sbs")
	require.Equal(t, 200, status)
	images := list["local_images"].([]any)
	require.NotEmpty(t, images)
	id := images[0].(map[string]any)["id"].(string)

	status, body := testutil.DoGet(t, ts, "/marketplace/v2/local-images/"+id)
	require.Equal(t, 200, status)
	require.Equal(t, id, body["id"])
	require.Equal(t, "ubuntu_noble", body["label"])
	require.Equal(t, "fr-par-1", body["zone"])
}

func TestMarketplaceLocalImageGetUnknownID404(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/marketplace/v2/local-images/00000000-0000-0000-0000-000000000000")
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
}

func TestMarketplaceLocalImagesPaginationIgnored(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, baseline := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=ubuntu_noble&zone=fr-par-1&type=instance_sbs")
	require.Equal(t, 200, status)

	status, paged := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=ubuntu_noble&zone=fr-par-1&type=instance_sbs&page=1")
	require.Equal(t, 200, status)
	require.Equal(t, baseline, paged)
}

func TestCreateServerNormalizesImageLabelToObject(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, list := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=ubuntu_noble&zone=fr-par-1&type=instance_sbs")
	require.Equal(t, 200, status)
	expectedID := list["local_images"].([]any)[0].(map[string]any)["id"].(string)

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "commercial_type": "DEV1-S", "image": "ubuntu_noble"},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	image := server["image"].(map[string]any)
	require.Equal(t, expectedID, image["id"])
	require.Equal(t, "ubuntu_noble", image["name"])
	require.Equal(t, "x86_64", image["arch"])

	status, body = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/"+server["id"].(string))
	require.Equal(t, 200, status)
	server = unwrapInstanceResource(body)
	image = server["image"].(map[string]any)
	require.Equal(t, expectedID, image["id"])
}

func TestCreateServerNormalizesImageUUIDToObject(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	imageID := "11111111-1111-1111-1111-111111111111"
	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "commercial_type": "DEV1-S", "image": imageID},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	image := server["image"].(map[string]any)
	require.Equal(t, imageID, image["id"])
	require.Equal(t, imageID, image["name"])
}

func TestCreateServerOverridesMalformedPublicIPFields(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{
			"name":       "web-1",
			"public_ips": "bad-type",
			"public_ip":  "bad-type",
		},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	publicIPs, ok := server["public_ips"].([]any)
	require.True(t, ok)
	require.Len(t, publicIPs, 0)
	require.Contains(t, server, "public_ip")
	require.Nil(t, server["public_ip"])
}

func TestCreateServerResolvesPublicIPFromIPsTable(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create a reserved IP first.
	status, ipBody := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/ips",
		map[string]any{},
	)
	require.Equal(t, 200, status)
	ipData := ipBody["ip"].(map[string]any)
	ipID := ipData["id"].(string)
	ipAddr := ipData["address"].(string)

	// Create server referencing that IP via public_ips array (SDK format).
	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{
			"name":       "web-ip",
			"public_ips": []any{ipID},
		},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)

	pubIP, ok := server["public_ip"].(map[string]any)
	require.True(t, ok, "public_ip should be an object, got %T", server["public_ip"])
	require.Equal(t, ipID, pubIP["id"])
	require.Equal(t, ipAddr, pubIP["address"])

	pubIPs, ok := server["public_ips"].([]any)
	require.True(t, ok)
	require.Len(t, pubIPs, 1)
}

func TestCreateServerInjectsDefaultRootVolume(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1"},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)

	volumes, ok := server["volumes"].(map[string]any)
	require.True(t, ok)
	rootRaw, ok := volumes["0"]
	require.True(t, ok)
	root := rootRaw.(map[string]any)
	require.NotEmpty(t, root["id"])
	require.Equal(t, "web-1-vol-0", root["name"])
	require.Equal(t, float64(20000000000), root["size"])
	require.Equal(t, "l_ssd", root["volume_type"])
	require.Equal(t, "available", root["state"])
	require.Equal(t, true, root["boot"])
	require.Equal(t, "fr-par-1", root["zone"])

	status, body = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/"+server["id"].(string))
	require.Equal(t, 200, status)
	server = unwrapInstanceResource(body)
	volumes, ok = server["volumes"].(map[string]any)
	require.True(t, ok)
	_, ok = volumes["0"]
	require.True(t, ok)
}

func TestCreateServerOverridesProvidedVolumesWithRootVolume(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "volumes": map[string]any{}},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)

	volumes, ok := server["volumes"].(map[string]any)
	require.True(t, ok)
	rootRaw, ok := volumes["0"]
	require.True(t, ok)
	root := rootRaw.(map[string]any)
	require.NotEmpty(t, root["id"])
	require.Equal(t, "web-1-vol-0", root["name"])
}

func TestGetVolumeFromServerRootVolume(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1"},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	volumes := server["volumes"].(map[string]any)
	root := volumes["0"].(map[string]any)
	volumeID := root["id"].(string)

	status, body = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/volumes/"+volumeID)
	require.Equal(t, 200, status)
	volume := body["volume"].(map[string]any)
	require.Equal(t, volumeID, volume["id"])
	require.Equal(t, "web-1-vol-0", volume["name"])
	require.Equal(t, "fr-par-1", volume["zone"])
}

func TestGetVolumeNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/volumes/non-existent")
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
}

func TestDeleteVolumeReturns204(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status := testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/volumes/non-existent")
	require.Equal(t, 204, status)
}

func TestDeleteVolumeAfterServerDelete(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1"},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	serverID := server["id"].(string)
	volumeID := server["volumes"].(map[string]any)["0"].(map[string]any)["id"].(string)

	status = testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID)
	require.Equal(t, 204, status)

	status = testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/volumes/"+volumeID)
	require.Equal(t, 204, status)
}

func TestDeleteServerDetachesIP(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "web-1"})
	require.Equal(t, 200, status)
	serverID := resourceID(server)

	status, ip := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/ips", map[string]any{"server_id": serverID})
	require.Equal(t, 200, status)
	ipID := resourceID(ip)

	status = testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID)
	require.Equal(t, 204, status)

	status, ip = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/ips/"+ipID)
	require.Equal(t, 200, status)
	got := unwrapInstanceResource(ip)
	require.Contains(t, got, "server_id")
	require.Nil(t, got["server_id"])
}

func TestDeleteServerCascadesPrivateNICs(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "vpc"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{"name": "pn", "vpc_id": vpc["id"]})
	_, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "web-1"})
	serverID := resourceID(server)
	testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics", map[string]any{"private_network_id": pn["id"]})

	status := testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID)
	require.Equal(t, 204, status)

	status, body := testutil.DoList(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])
}

func TestServerTerminateActionDeletesServer(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "web-term"})
	require.Equal(t, 200, status)
	serverID := resourceID(server)

	// Terminate via action endpoint (what the Scaleway provider calls during destroy).
	status, resp := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID+"/action", map[string]any{"action": "terminate"})
	require.Equal(t, 200, status)
	task := resp["task"].(map[string]any)
	require.Equal(t, "terminate", task["description"])

	// Server should be gone.
	status, _ = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID)
	require.Equal(t, 404, status)
}

func TestDeleteSecurityGroupDetachesServer(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, sg := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/security_groups", map[string]any{"name": "sg-1"})
	require.Equal(t, 200, status)
	sgID := resourceID(sg)

	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "web-1", "security_group": sgID})
	require.Equal(t, 200, status)
	serverID := resourceID(server)

	status = testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/security_groups/"+sgID)
	require.Equal(t, 204, status)

	status, server = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID)
	require.Equal(t, 200, status)
	got := unwrapInstanceResource(server)
	require.Contains(t, got, "security_group")
	require.Nil(t, got["security_group"])
	require.Contains(t, got, "security_group_id")
	require.Nil(t, got["security_group_id"])
}

func TestCreateServerNormalizesSecurityGroupStringToObject(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, sgResp := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups",
		map[string]any{"name": "sg-1"},
	)
	require.Equal(t, 200, status)
	sgID := resourceID(sgResp)

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "security_group": sgID},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	sg := server["security_group"].(map[string]any)
	require.Equal(t, sgID, sg["id"])
	require.Equal(t, "", sg["name"])

	status, body = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/"+server["id"].(string))
	require.Equal(t, 200, status)
	server = unwrapInstanceResource(body)
	sg = server["security_group"].(map[string]any)
	require.Equal(t, sgID, sg["id"])
}

func TestCreateServerRejectsUnknownSecurityGroupReference(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "security_group": "non-existent-sg"},
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "referenced resource not found", body["message"])
}

func TestCreateServerNormalizesSecurityGroupMapMissingName(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, sgResp := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/security_groups", map[string]any{"name": "sg-1"})
	require.Equal(t, 200, status)
	sgID := resourceID(sgResp)

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{
			"name":           "web-1",
			"security_group": map[string]any{"id": sgID},
		},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	sg := server["security_group"].(map[string]any)
	require.Equal(t, sgID, sg["id"])
	require.Equal(t, "", sg["name"])
	require.Equal(t, sgID, server["security_group_id"])
}

func TestCreateServerNormalizesSecurityGroupIDOnlyInput(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, sgResp := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/security_groups", map[string]any{"name": "sg-1"})
	require.Equal(t, 200, status)
	sgID := resourceID(sgResp)

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "security_group_id": sgID},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	sg := server["security_group"].(map[string]any)
	require.Equal(t, sgID, sg["id"])
	require.Equal(t, "", sg["name"])
	require.Equal(t, sgID, server["security_group_id"])
}

func TestCreateServerClearsInvalidSecurityGroupType(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{
			"name":              "web-1",
			"security_group":    float64(123),
			"security_group_id": "ignored",
		},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	// Provider dereferences SecurityGroup.ID without nil check, so a default is always injected.
	sg, ok := server["security_group"].(map[string]any)
	require.True(t, ok, "security_group should be an object")
	require.NotEmpty(t, sg["id"])
}

func TestCreateServerEmptySecurityGroupStringClearsReference(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, sgResp := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/security_groups", map[string]any{"name": "sg-1"})
	require.Equal(t, 200, status)
	sgID := resourceID(sgResp)

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{
			"name":              "web-1",
			"security_group":    "   ",
			"security_group_id": sgID,
		},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	// Provider dereferences SecurityGroup.ID without nil check, so a default is always injected.
	sg, ok := server["security_group"].(map[string]any)
	require.True(t, ok, "security_group should be an object")
	require.NotEmpty(t, sg["id"])
}

func TestCreateServerRejectsUnknownSecurityGroupIDOnly(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "security_group_id": "non-existent-sg"},
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "referenced resource not found", body["message"])
}

func TestInstanceServerLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "commercial_type": "DEV1-S"},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	serverID := server["id"].(string)
	require.NotEmpty(t, serverID)
	publicIPs, ok := server["public_ips"].([]any)
	require.True(t, ok)
	require.Len(t, publicIPs, 0)
	require.Contains(t, server, "public_ip")
	require.Nil(t, server["public_ip"])

	status, body = testutil.DoGet(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID,
	)
	require.Equal(t, 200, status)
	server = unwrapInstanceResource(body)
	require.Equal(t, "web-1", server["name"])
	publicIPs, ok = server["public_ips"].([]any)
	require.True(t, ok)
	require.Len(t, publicIPs, 0)
	require.Contains(t, server, "public_ip")
	require.Nil(t, server["public_ip"])

	status, body = testutil.DoList(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
	)
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), body["total_count"])

	status = testutil.DoDelete(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID,
	)
	require.Equal(t, 204, status)

	status, _ = testutil.DoGet(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID,
	)
	require.Equal(t, 404, status)
}

func TestServerUserDataListEmpty(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1"},
	)
	require.Equal(t, 200, status)
	serverID := resourceID(body)

	status, body = testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID+"/user_data")
	require.Equal(t, 200, status)
	userData, ok := body["user_data"].([]any)
	require.True(t, ok)
	require.Len(t, userData, 0)
}

func TestServerUserDataListNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/non-existent/user_data")
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
}

func TestServerUserDataPatchAccepted(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1"},
	)
	require.Equal(t, 200, status)
	serverID := resourceID(body)

	status, _ = doPatch(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/user_data/cloud-init",
		"#!/bin/bash\necho hello",
	)
	require.Equal(t, 204, status)
}

func TestServerUserDataPatchNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := doPatch(t, ts,
		"/instance/v1/zones/fr-par-1/servers/non-existent/user_data/cloud-init",
		"#!/bin/bash",
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
}

func TestServerActionAccepted(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, created := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1"},
	)
	require.Equal(t, 200, status)
	serverID := resourceID(created)

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/action",
		map[string]any{"action": "poweroff"},
	)
	require.Equal(t, 200, status)
	task := body["task"].(map[string]any)
	require.NotEmpty(t, task["id"])
	require.Equal(t, "poweroff", task["description"])
	require.Equal(t, float64(100), task["progress"])
	require.Equal(t, "success", task["status"])
}

func TestServerActionNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/non-existent/action",
		map[string]any{"action": "poweroff"},
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
}

func TestCrossServiceFlow(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts,
		"/vpc/v1/regions/fr-par/vpcs",
		map[string]any{"name": "main"},
	)
	_, pn := testutil.DoCreate(t, ts,
		"/vpc/v1/regions/fr-par/private-networks",
		map[string]any{"name": "app-net", "vpc_id": vpc["id"]},
	)
	_, srv := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "commercial_type": "DEV1-S"},
	)
	_, nic := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+resourceID(srv)+"/private_nics",
		map[string]any{"private_network_id": pn["id"]},
	)

	state := testutil.GetState(t, ts)
	instance := state["instance"].(map[string]any)
	nics := instance["private_nics"].([]any)
	require.Len(t, nics, 1)
	require.Equal(t, resourceID(nic), nics[0].(map[string]any)["id"])
}

func TestFKRejectionHTTP(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/nonexistent/private_nics",
		map[string]any{"private_network_id": "also-nonexistent"},
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "referenced resource not found", body["message"])

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks",
		map[string]any{"name": "pn", "vpc_id": vpc["id"]})
	status = testutil.DoDelete(t, ts, "/vpc/v1/regions/fr-par/vpcs/"+vpc["id"].(string))
	require.Equal(t, 409, status)
}

func TestSecurityGroupPatchLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, created := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups",
		map[string]any{"name": "sg", "inbound_default_policy": "drop"},
	)
	require.Equal(t, 200, status)
	sgID := resourceID(created)

	status, patched := doPatch(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups/"+sgID,
		map[string]any{
			"inbound_default_policy":  "accept",
			"outbound_default_policy": "accept",
		},
	)
	require.Equal(t, 200, status)
	sg := unwrapInstanceResource(patched)
	require.Equal(t, sgID, sg["id"])
	require.Equal(t, "sg", sg["name"])
	require.Equal(t, "accept", sg["inbound_default_policy"])
	require.Equal(t, "accept", sg["outbound_default_policy"])

	status, got := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/security_groups/"+sgID)
	require.Equal(t, 200, status)
	sg = unwrapInstanceResource(got)
	require.Equal(t, "accept", sg["inbound_default_policy"])
	require.Equal(t, "accept", sg["outbound_default_policy"])
}

func TestSecurityGroupPatchNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := doPatch(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups/non-existent",
		map[string]any{"inbound_default_policy": "accept"},
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "resource not found", body["message"])
}

func TestSecurityGroupRulesPutLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, created := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups",
		map[string]any{"name": "sg"},
	)
	require.Equal(t, 200, status)
	sgID := resourceID(created)

	rules := []any{
		map[string]any{"action": "accept", "protocol": "TCP", "dest_port_from": 80},
	}
	status, body := doPut(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups/"+sgID+"/rules",
		map[string]any{"rules": rules},
	)
	require.Equal(t, 200, status)
	gotRules := body["rules"].([]any)
	require.Len(t, gotRules, 1)
	gotRule := gotRules[0].(map[string]any)
	require.Equal(t, "accept", gotRule["action"])
	require.Equal(t, "TCP", gotRule["protocol"])
	require.Equal(t, float64(80), gotRule["dest_port_from"])

	status, got := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/security_groups/"+sgID)
	require.Equal(t, 200, status)
	sg := unwrapInstanceResource(got)
	gotRules = sg["rules"].([]any)
	require.Len(t, gotRules, 1)
	gotRule = gotRules[0].(map[string]any)
	require.Equal(t, "accept", gotRule["action"])
	require.Equal(t, "TCP", gotRule["protocol"])
	require.Equal(t, float64(80), gotRule["dest_port_from"])
}

func TestSecurityGroupRulesPutNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := doPut(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups/non-existent/rules",
		map[string]any{"rules": []any{}},
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "resource not found", body["message"])
}

func TestSecurityGroupRulesGetAfterPut(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, created := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups",
		map[string]any{"name": "sg"},
	)
	require.Equal(t, 200, status)
	sgID := resourceID(created)

	rules := []any{
		map[string]any{"action": "accept", "protocol": "TCP", "dest_port_from": 443},
	}
	status, _ = doPut(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups/"+sgID+"/rules",
		map[string]any{"rules": rules},
	)
	require.Equal(t, 200, status)

	status, body := testutil.DoList(t, ts, "/instance/v1/zones/fr-par-1/security_groups/"+sgID+"/rules?page=1")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), body["total_count"])
	gotRules := body["rules"].([]any)
	require.Len(t, gotRules, 1)
	gotRule := gotRules[0].(map[string]any)
	require.Equal(t, "accept", gotRule["action"])
	require.Equal(t, "TCP", gotRule["protocol"])
	require.Equal(t, float64(443), gotRule["dest_port_from"])
}

func TestSecurityGroupRulesGetEmptyWhenUnset(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, created := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/security_groups",
		map[string]any{"name": "sg"},
	)
	require.Equal(t, 200, status)
	sgID := resourceID(created)

	status, body := testutil.DoList(t, ts, "/instance/v1/zones/fr-par-1/security_groups/"+sgID+"/rules")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])
	require.Len(t, body["rules"].([]any), 0)
}

func TestSecurityGroupRulesGetNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoList(t, ts, "/instance/v1/zones/fr-par-1/security_groups/non-existent/rules")
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "resource not found", body["message"])
}

func TestDeleteConflictForMultipleDependencies(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})
	_, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "s"})
	testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+resourceID(server)+"/private_nics", map[string]any{
		"private_network_id": pn["id"],
	})
	status := testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/servers/"+resourceID(server))
	require.Equal(t, 204, status)
	status, body := testutil.DoList(t, ts, "/instance/v1/zones/fr-par-1/servers/"+resourceID(server)+"/private_nics")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])

	// K8s cluster delete rejects when pools exist (409).
	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters", map[string]any{"name": "k"})
	_, pool := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters/"+cluster["id"].(string)+"/pools", map[string]any{"name": "p"})
	status = testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/clusters/"+cluster["id"].(string))
	require.Equal(t, 409, status)
	// Delete pool first, then cluster.
	testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/pools/"+pool["id"].(string))
	status = testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/clusters/"+cluster["id"].(string))
	require.Equal(t, 200, status)

	// RDB instance delete rejects when children exist (409).
	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{"name": "db"})
	testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+inst["id"].(string)+"/databases", map[string]any{"name": "appdb"})
	status = testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+inst["id"].(string))
	require.Equal(t, 409, status)
	// Delete database first, then instance.
	testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+inst["id"].(string)+"/databases/appdb")
	status = testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+inst["id"].(string))
	require.Equal(t, 204, status)
}

func TestUnknownServiceState404(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/mock/state/unknown")
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "unknown service", body["message"])

	status, body = testutil.DoGet(t, ts, "/mock/state/account")
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "unknown service", body["message"])
}

func TestRDBInitEndpointsValidationAndEnginePort(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})

	status, body := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":   "mysql-db",
		"engine": "MySQL-8",
		"init_endpoints": []any{
			map[string]any{
				"private_network": map[string]any{"id": pn["id"]},
			},
		},
	})
	require.Equal(t, 200, status)
	endpoints := body["endpoints"].([]any)
	require.Len(t, endpoints, 1)
	ep := endpoints[0].(map[string]any)
	require.Equal(t, float64(3306), ep["port"])
	require.Equal(t, pn["id"], ep["private_network"].(map[string]any)["id"])

	status, body = testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":   "bad-pn-db",
		"engine": "PostgreSQL-15",
		"init_endpoints": []any{
			map[string]any{
				"private_network": map[string]any{"id": "non-existent-pn"},
			},
		},
	})
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "referenced resource not found", body["message"])

	status, body = testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":   "public-mysql",
		"engine": "MySQL-8",
	})
	require.Equal(t, 200, status)
	endpoints = body["endpoints"].([]any)
	require.Len(t, endpoints, 1)
	ep = endpoints[0].(map[string]any)
	require.Equal(t, float64(3306), ep["port"])
}

func TestAdminResetAndStateShape(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	testutil.ResetState(t, ts)

	state := testutil.GetState(t, ts)
	require.Contains(t, state, "instance")
	require.Contains(t, state, "vpc")
	require.Contains(t, state, "lb")
	require.Contains(t, state, "k8s")
	require.Contains(t, state, "rdb")
	require.Contains(t, state, "iam")

	instance := state["instance"].(map[string]any)
	require.Len(t, instance["servers"].([]any), 0)
	vpc := state["vpc"].(map[string]any)
	require.Len(t, vpc["vpcs"].([]any), 0)
}

func TestIAMApplicationLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts, "/iam/v1alpha1/applications", map[string]any{"name": "app"})
	require.Equal(t, 200, status)
	appID := body["id"].(string)

	status, body = testutil.DoGet(t, ts, "/iam/v1alpha1/applications/"+appID)
	require.Equal(t, 200, status)
	require.Equal(t, appID, body["id"])

	status, body = testutil.DoList(t, ts, "/iam/v1alpha1/applications")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), body["total_count"])

	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/applications/"+appID)
	require.Equal(t, 204, status)
}

func TestIAMAPIKeyLifecycleAndRules(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, app := testutil.DoCreate(t, ts, "/iam/v1alpha1/applications", map[string]any{"name": "app"})
	status, key := testutil.DoCreate(t, ts, "/iam/v1alpha1/api-keys", map[string]any{"application_id": app["id"]})
	require.Equal(t, 200, status)
	accessKey := key["access_key"].(string)
	require.NotEmpty(t, key["secret_key"])

	status, got := testutil.DoGet(t, ts, "/iam/v1alpha1/api-keys/"+accessKey)
	require.Equal(t, 200, status)
	_, hasSecret := got["secret_key"]
	require.False(t, hasSecret)
	require.Equal(t, app["id"], got["application_id"])

	status, list := testutil.DoList(t, ts, "/iam/v1alpha1/api-keys")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), list["total_count"])
	item := list["api_keys"].([]any)[0].(map[string]any)
	_, hasSecret = item["secret_key"]
	require.False(t, hasSecret)

	status, userKey := testutil.DoCreate(t, ts, "/iam/v1alpha1/api-keys", map[string]any{"user_id": "user-1"})
	require.Equal(t, 200, status)
	require.Equal(t, "user-1", userKey["user_id"])

	status, body := testutil.DoCreate(t, ts, "/iam/v1alpha1/api-keys", map[string]any{"application_id": "non-existent"})
	require.Equal(t, 404, status)
	require.Equal(t, "referenced resource not found", body["message"])

	status, body = testutil.DoCreate(t, ts, "/iam/v1alpha1/api-keys", map[string]any{
		"application_id": app["id"],
		"user_id":        "user-1",
	})
	require.Equal(t, 400, status)
	require.Equal(t, "invalid_argument", body["type"])

	status, body = testutil.DoCreate(t, ts, "/iam/v1alpha1/api-keys", map[string]any{})
	require.Equal(t, 400, status)
	require.Equal(t, "invalid_argument", body["type"])

	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/applications/"+app["id"].(string))
	require.Equal(t, 409, status)

	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/api-keys/"+accessKey)
	require.Equal(t, 204, status)
}

func TestIAMPolicyLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, app := testutil.DoCreate(t, ts, "/iam/v1alpha1/applications", map[string]any{"name": "app"})
	status, pol := testutil.DoCreate(t, ts, "/iam/v1alpha1/policies", map[string]any{"name": "p1", "application_id": app["id"]})
	require.Equal(t, 200, status)
	polID := pol["id"].(string)

	status, _ = testutil.DoCreate(t, ts, "/iam/v1alpha1/policies", map[string]any{"name": "p2"})
	require.Equal(t, 200, status)

	status, body := testutil.DoGet(t, ts, "/iam/v1alpha1/policies/"+polID)
	require.Equal(t, 200, status)
	require.Equal(t, polID, body["id"])

	status, body = testutil.DoList(t, ts, "/iam/v1alpha1/policies")
	require.Equal(t, 200, status)
	require.Equal(t, float64(2), body["total_count"])

	status, body = testutil.DoCreate(t, ts, "/iam/v1alpha1/policies", map[string]any{
		"name":           "bad",
		"application_id": "non-existent",
	})
	require.Equal(t, 404, status)
	require.Equal(t, "referenced resource not found", body["message"])

	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/policies/"+polID)
	require.Equal(t, 204, status)
}

func TestIAMRulesListEndpoint(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, pol := testutil.DoCreate(t, ts, "/iam/v1alpha1/policies", map[string]any{"name": "p1"})
	policyID := pol["id"].(string)

	status, body := testutil.DoList(t, ts, "/iam/v1alpha1/rules?policy_id="+policyID)
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])
	require.Len(t, body["rules"].([]any), 0)

	status, body = testutil.DoList(t, ts, "/iam/v1alpha1/rules")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])
	require.Len(t, body["rules"].([]any), 0)
}

func TestIAMAndAccountSSHKeyCompatibility(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, fromAccount := testutil.DoCreate(t, ts, "/account/v2alpha1/ssh-keys", map[string]any{
		"name":       "legacy",
		"public_key": "ssh-ed25519 AAAAlegacy",
	})
	require.Equal(t, 200, status)
	keyID := fromAccount["id"].(string)

	status, _ = testutil.DoGet(t, ts, "/iam/v1alpha1/ssh-keys/"+keyID)
	require.Equal(t, 200, status)

	status, fromIAM := testutil.DoCreate(t, ts, "/iam/v1alpha1/ssh-keys", map[string]any{
		"name":       "new",
		"public_key": "ssh-ed25519 AAAAnew",
	})
	require.Equal(t, 200, status)
	otherID := fromIAM["id"].(string)

	status, _ = testutil.DoGet(t, ts, "/account/v2alpha1/ssh-keys/"+otherID)
	require.Equal(t, 200, status)

	status, listIAM := testutil.DoList(t, ts, "/iam/v1alpha1/ssh-keys")
	require.Equal(t, 200, status)
	status, listAccount := testutil.DoList(t, ts, "/account/v2alpha1/ssh-keys")
	require.Equal(t, 200, status)
	require.Equal(t, listIAM["total_count"], listAccount["total_count"])

	status = testutil.DoDelete(t, ts, "/account/v2alpha1/ssh-keys/"+keyID)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/ssh-keys/"+otherID)
	require.Equal(t, 204, status)
}

func TestIAMServiceState(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, app := testutil.DoCreate(t, ts, "/iam/v1alpha1/applications", map[string]any{"name": "app"})
	testutil.DoCreate(t, ts, "/iam/v1alpha1/api-keys", map[string]any{"application_id": app["id"]})
	testutil.DoCreate(t, ts, "/iam/v1alpha1/policies", map[string]any{"name": "pol"})
	testutil.DoCreate(t, ts, "/iam/v1alpha1/ssh-keys", map[string]any{"name": "k", "public_key": "ssh-ed25519 AAAA"})

	status, body := testutil.DoGet(t, ts, "/mock/state/iam")
	require.Equal(t, 200, status)
	require.Contains(t, body, "applications")
	require.Contains(t, body, "api_keys")
	require.Contains(t, body, "policies")
	require.Contains(t, body, "ssh_keys")
	apiKeys := body["api_keys"].([]any)
	require.NotEmpty(t, apiKeys)
	_, hasSecret := apiKeys[0].(map[string]any)["secret_key"]
	require.False(t, hasSecret)

	// Full state should also include IAM API keys without secret_key.
	status, full := testutil.DoGet(t, ts, "/mock/state")
	require.Equal(t, 200, status)
	iam := full["iam"].(map[string]any)
	keys := iam["api_keys"].([]any)
	require.NotEmpty(t, keys)
	_, hasSecret = keys[0].(map[string]any)["secret_key"]
	require.False(t, hasSecret)
}

func TestServiceStateAllServices(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Seed one resource per service.
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{"name": "pn", "vpc_id": vpc["id"]})
	_, srv := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "s"})
	testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+resourceID(srv)+"/private_nics", map[string]any{"private_network_id": pn["id"]})
	testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters", map[string]any{"name": "k"})
	testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters/"+cluster["id"].(string)+"/pools", map[string]any{"name": "p"})
	testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{"name": "db", "engine": "PostgreSQL-15"})
	testutil.DoCreate(t, ts, "/iam/v1alpha1/applications", map[string]any{"name": "app"})

	for _, svc := range []string{"instance", "vpc", "lb", "k8s", "rdb", "iam"} {
		t.Run(svc, func(t *testing.T) {
			status, body := testutil.DoGet(t, ts, "/mock/state/"+svc)
			require.Equal(t, 200, status)
			require.NotEmpty(t, body)
		})
	}
}

func TestDeleteNonExistentReturns404(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	paths := []string{
		"/instance/v1/zones/fr-par-1/servers/non-existent",
		"/instance/v1/zones/fr-par-1/ips/non-existent",
		"/instance/v1/zones/fr-par-1/security_groups/non-existent",
		"/vpc/v1/regions/fr-par/vpcs/non-existent",
		"/vpc/v1/regions/fr-par/private-networks/non-existent",
		"/lb/v1/zones/fr-par-1/lbs/non-existent",
		"/lb/v1/zones/fr-par-1/frontends/non-existent",
		"/lb/v1/zones/fr-par-1/backends/non-existent",
		"/k8s/v1/regions/fr-par/clusters/non-existent",
		"/k8s/v1/regions/fr-par/pools/non-existent",
		"/rdb/v1/regions/fr-par/instances/non-existent",
		"/iam/v1alpha1/applications/non-existent",
		"/iam/v1alpha1/api-keys/non-existent",
		"/iam/v1alpha1/policies/non-existent",
		"/iam/v1alpha1/ssh-keys/non-existent",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			status := testutil.DoDelete(t, ts, p)
			require.Equal(t, 404, status)
		})
	}
}

func TestServiceStateSuccessPath(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "s"})
	status, body := testutil.DoGet(t, ts, "/mock/state/instance")
	require.Equal(t, 200, status)
	require.Contains(t, body, "servers")
	require.Contains(t, body, "ips")
	require.Contains(t, body, "private_nics")
	require.Contains(t, body, "security_groups")
}

func TestResourceLifecyclesTableDriven(t *testing.T) {
	type lifecycleCase struct {
		name         string
		setup        func(t *testing.T, ts *httptest.Server, ctx map[string]string)
		createPath   string
		listPath     string
		getPath      string
		deletePath   string
		listKey      string
		body         map[string]any
		idField      string
		deleteStatus int // 0 means default 204
	}

	cases := []lifecycleCase{
		{
			name:       "InstanceIPs",
			setup:      setupServer,
			createPath: "/instance/v1/zones/{zone}/ips",
			listPath:   "/instance/v1/zones/{zone}/ips",
			getPath:    "/instance/v1/zones/{zone}/ips/{id}",
			deletePath: "/instance/v1/zones/{zone}/ips/{id}",
			listKey:    "ips",
			body:       map[string]any{"server_id": "{server_id}"},
			idField:    "id",
		},
		{
			name:       "SecurityGroups",
			createPath: "/instance/v1/zones/{zone}/security_groups",
			listPath:   "/instance/v1/zones/{zone}/security_groups",
			getPath:    "/instance/v1/zones/{zone}/security_groups/{id}",
			deletePath: "/instance/v1/zones/{zone}/security_groups/{id}",
			listKey:    "security_groups",
			body:       map[string]any{"name": "sg-1"},
			idField:    "id",
		},
		{
			name:       "PrivateNICs",
			setup:      setupServerAndPrivateNetwork,
			createPath: "/instance/v1/zones/{zone}/servers/{server_id}/private_nics",
			listPath:   "/instance/v1/zones/{zone}/servers/{server_id}/private_nics",
			getPath:    "/instance/v1/zones/{zone}/servers/{server_id}/private_nics/{id}",
			deletePath: "/instance/v1/zones/{zone}/servers/{server_id}/private_nics/{id}",
			listKey:    "private_nics",
			body:       map[string]any{"private_network_id": "{pn_id}"},
			idField:    "id",
		},
		{
			name:       "VPCGetListDelete",
			createPath: "/vpc/v1/regions/{region}/vpcs",
			listPath:   "/vpc/v1/regions/{region}/vpcs",
			getPath:    "/vpc/v1/regions/{region}/vpcs/{id}",
			deletePath: "/vpc/v1/regions/{region}/vpcs/{id}",
			listKey:    "vpcs",
			body:       map[string]any{"name": "main"},
			idField:    "id",
		},
		{
			name:       "PrivateNetworkGetListDelete",
			setup:      setupVPC,
			createPath: "/vpc/v1/regions/{region}/private-networks",
			listPath:   "/vpc/v1/regions/{region}/private-networks",
			getPath:    "/vpc/v1/regions/{region}/private-networks/{id}",
			deletePath: "/vpc/v1/regions/{region}/private-networks/{id}",
			listKey:    "private_networks",
			body:       map[string]any{"name": "pn", "vpc_id": "{vpc_id}"},
			idField:    "id",
		},
		{
			name:       "LoadBalancers",
			createPath: "/lb/v1/zones/{zone}/lbs",
			listPath:   "/lb/v1/zones/{zone}/lbs",
			getPath:    "/lb/v1/zones/{zone}/lbs/{id}",
			deletePath: "/lb/v1/zones/{zone}/lbs/{id}",
			listKey:    "lbs",
			body:       map[string]any{"name": "lb"},
			idField:    "id",
		},
		{
			name:       "Frontends",
			setup:      setupLB,
			createPath: "/lb/v1/zones/{zone}/frontends",
			listPath:   "/lb/v1/zones/{zone}/frontends",
			getPath:    "/lb/v1/zones/{zone}/frontends/{id}",
			deletePath: "/lb/v1/zones/{zone}/frontends/{id}",
			listKey:    "frontends",
			body:       map[string]any{"name": "http", "lb_id": "{lb_id}"},
			idField:    "id",
		},
		{
			name:       "Backends",
			setup:      setupLB,
			createPath: "/lb/v1/zones/{zone}/backends",
			listPath:   "/lb/v1/zones/{zone}/backends",
			getPath:    "/lb/v1/zones/{zone}/backends/{id}",
			deletePath: "/lb/v1/zones/{zone}/backends/{id}",
			listKey:    "backends",
			body:       map[string]any{"name": "be", "lb_id": "{lb_id}"},
			idField:    "id",
		},
		{
			name:         "K8sClusters",
			createPath:   "/k8s/v1/regions/{region}/clusters",
			listPath:     "/k8s/v1/regions/{region}/clusters",
			getPath:      "/k8s/v1/regions/{region}/clusters/{id}",
			deletePath:   "/k8s/v1/regions/{region}/clusters/{id}",
			listKey:      "clusters",
			body:         map[string]any{"name": "k"},
			idField:      "id",
			deleteStatus: 200,
		},
		{
			name:         "K8sPools",
			setup:        setupCluster,
			createPath:   "/k8s/v1/regions/{region}/clusters/{cluster_id}/pools",
			listPath:     "/k8s/v1/regions/{region}/clusters/{cluster_id}/pools",
			getPath:      "/k8s/v1/regions/{region}/pools/{id}",
			deletePath:   "/k8s/v1/regions/{region}/pools/{id}",
			listKey:      "pools",
			body:         map[string]any{"name": "pool"},
			idField:      "id",
			deleteStatus: 200,
		},
		{
			name:       "RDBInstances",
			createPath: "/rdb/v1/regions/{region}/instances",
			listPath:   "/rdb/v1/regions/{region}/instances",
			getPath:    "/rdb/v1/regions/{region}/instances/{id}",
			deletePath: "/rdb/v1/regions/{region}/instances/{id}",
			listKey:    "instances",
			body:       map[string]any{"name": "rdb", "engine": "PostgreSQL-15"},
			idField:    "id",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ts, cleanup := testutil.NewTestServer(t)
			defer cleanup()

			ctx := map[string]string{"zone": "fr-par-1", "region": "fr-par"}
			if tt.setup != nil {
				tt.setup(t, ts, ctx)
			}

			status, body := testutil.DoCreate(t, ts, pathWithCtx(tt.createPath, ctx), bodyWithCtx(tt.body, ctx))
			require.Equal(t, 200, status)
			id := resourceField(body, tt.idField).(string)
			require.NotEmpty(t, id)
			ctx["id"] = id

			status, body = testutil.DoGet(t, ts, pathWithCtx(tt.getPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, id, resourceField(body, tt.idField))

			status, body = testutil.DoList(t, ts, pathWithCtx(tt.listPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, float64(1), body["total_count"])
			require.Len(t, body[tt.listKey].([]any), 1)

			status = testutil.DoDelete(t, ts, pathWithCtx(tt.deletePath, ctx))
			expectedDeleteStatus := tt.deleteStatus
			if expectedDeleteStatus == 0 {
				expectedDeleteStatus = 204
			}
			require.Equal(t, expectedDeleteStatus, status)

			status, _ = testutil.DoGet(t, ts, pathWithCtx(tt.getPath, ctx))
			require.Equal(t, 404, status)
		})
	}
}

func TestResourceLifecyclesWithoutGet(t *testing.T) {
	type noGetCase struct {
		name         string
		setup        func(t *testing.T, ts *httptest.Server, ctx map[string]string)
		createPath   string
		listPath     string
		deletePath   string
		listKey      string
		body         map[string]any
		deleteIDFrom string
	}

	cases := []noGetCase{
		{
			name:         "LBPrivateNetworkAttachment",
			setup:        setupLBAndPrivateNetwork,
			createPath:   "/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks",
			listPath:     "/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks",
			deletePath:   "/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks/{delete_id}",
			listKey:      "private_network",
			body:         map[string]any{"private_network_id": "{pn_id}"},
			deleteIDFrom: "private_network_id",
		},
		{
			name:         "RDBDatabases",
			setup:        setupRDBInstance,
			createPath:   "/rdb/v1/regions/{region}/instances/{instance_id}/databases",
			listPath:     "/rdb/v1/regions/{region}/instances/{instance_id}/databases",
			deletePath:   "/rdb/v1/regions/{region}/instances/{instance_id}/databases/{delete_id}",
			listKey:      "databases",
			body:         map[string]any{"name": "appdb"},
			deleteIDFrom: "name",
		},
		{
			name:         "RDBUsers",
			setup:        setupRDBInstance,
			createPath:   "/rdb/v1/regions/{region}/instances/{instance_id}/users",
			listPath:     "/rdb/v1/regions/{region}/instances/{instance_id}/users",
			deletePath:   "/rdb/v1/regions/{region}/instances/{instance_id}/users/{delete_id}",
			listKey:      "users",
			body:         map[string]any{"name": "appuser"},
			deleteIDFrom: "name",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ts, cleanup := testutil.NewTestServer(t)
			defer cleanup()

			ctx := map[string]string{"zone": "fr-par-1", "region": "fr-par"}
			if tt.setup != nil {
				tt.setup(t, ts, ctx)
			}

			status, body := testutil.DoCreate(t, ts, pathWithCtx(tt.createPath, ctx), bodyWithCtx(tt.body, ctx))
			require.Equal(t, 200, status)
			deleteID := body[tt.deleteIDFrom].(string)
			ctx["delete_id"] = deleteID

			status, body = testutil.DoList(t, ts, pathWithCtx(tt.listPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, float64(1), body["total_count"])
			require.Len(t, body[tt.listKey].([]any), 1)

			status = testutil.DoDelete(t, ts, pathWithCtx(tt.deletePath, ctx))
			require.Equal(t, 204, status)

			status, body = testutil.DoList(t, ts, pathWithCtx(tt.listPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, float64(0), body["total_count"])
		})
	}
}

func pathWithCtx(path string, ctx map[string]string) string {
	out := path
	for k, v := range ctx {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

func bodyWithCtx(body map[string]any, ctx map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		switch s := v.(type) {
		case string:
			out[k] = pathWithCtx(s, ctx)
		default:
			out[k] = v
		}
	}
	return out
}

func setupVPC(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/"+ctx["region"]+"/vpcs", map[string]any{"name": "vpc"})
	ctx["vpc_id"] = vpc["id"].(string)
}

func setupServer(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, srv := testutil.DoCreate(t, ts, "/instance/v1/zones/"+ctx["zone"]+"/servers", map[string]any{"name": "srv"})
	ctx["server_id"] = resourceID(srv)
}

func unwrapInstanceResource(body map[string]any) map[string]any {
	if resource, ok := body["server"].(map[string]any); ok {
		return resource
	}
	if resource, ok := body["ip"].(map[string]any); ok {
		return resource
	}
	if resource, ok := body["security_group"].(map[string]any); ok {
		return resource
	}
	if resource, ok := body["private_nic"].(map[string]any); ok {
		return resource
	}
	return body
}

func resourceField(body map[string]any, field string) any {
	if v, ok := body[field]; ok {
		return v
	}
	return unwrapInstanceResource(body)[field]
}

func resourceID(body map[string]any) string {
	return resourceField(body, "id").(string)
}

func doPatch(t *testing.T, ts *httptest.Server, path string, body any) (int, map[string]any) {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPatch, ts.URL+path, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	out := map[string]any{}
	if resp.StatusCode != http.StatusNoContent {
		err = json.NewDecoder(resp.Body).Decode(&out)
		require.NoError(t, err)
	}
	return resp.StatusCode, out
}

func doPut(t *testing.T, ts *httptest.Server, path string, body any) (int, map[string]any) {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPut, ts.URL+path, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	out := map[string]any{}
	if resp.StatusCode != http.StatusNoContent {
		err = json.NewDecoder(resp.Body).Decode(&out)
		require.NoError(t, err)
	}
	return resp.StatusCode, out
}

func setupLB(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/"+ctx["zone"]+"/lbs", map[string]any{"name": "lb"})
	ctx["lb_id"] = lb["id"].(string)
}

func setupCluster(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/"+ctx["region"]+"/clusters", map[string]any{"name": "cluster"})
	ctx["cluster_id"] = cluster["id"].(string)
}

func setupRDBInstance(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/"+ctx["region"]+"/instances", map[string]any{"name": "rdb", "engine": "PostgreSQL-15"})
	ctx["instance_id"] = inst["id"].(string)
}

func setupServerAndPrivateNetwork(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	setupVPC(t, ts, ctx)
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/"+ctx["region"]+"/private-networks", map[string]any{
		"name": "pn", "vpc_id": ctx["vpc_id"],
	})
	ctx["pn_id"] = pn["id"].(string)
	setupServer(t, ts, ctx)
}

func setupLBAndPrivateNetwork(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	setupLB(t, ts, ctx)
	setupVPC(t, ts, ctx)
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/"+ctx["region"]+"/private-networks", map[string]any{
		"name": "pn", "vpc_id": ctx["vpc_id"],
	})
	ctx["pn_id"] = pn["id"].(string)
}

// --- Tests for bug fixes that prevent TF provider panics ---

func TestRDBInstanceHasProviderRequiredFields(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":   "pg-db",
		"engine": "PostgreSQL-15",
	})
	require.Equal(t, 200, status)

	// Fields the TF provider's ResourceRdbInstanceRead dereferences.
	vol := body["volume"].(map[string]any)
	require.Equal(t, "lssd", vol["type"])
	require.Equal(t, float64(10000000000), vol["size"])

	bs := body["backup_schedule"].(map[string]any)
	require.Equal(t, false, bs["disabled"])
	require.Equal(t, float64(24), bs["frequency"])
	require.Equal(t, float64(7), bs["retention"])

	require.Equal(t, false, body["backup_same_region"])

	enc := body["encryption"].(map[string]any)
	require.Equal(t, false, enc["enabled"])

	require.IsType(t, []any{}, body["settings"])
	require.IsType(t, []any{}, body["init_settings"])

	lp := body["logs_policy"].(map[string]any)
	require.Equal(t, float64(30), lp["max_age_retention"])

	require.IsType(t, []any{}, body["tags"])
	require.IsType(t, []any{}, body["upgradable_version"])
	require.NotEmpty(t, body["organization_id"])
	require.NotEmpty(t, body["project_id"])
	require.IsType(t, []any{}, body["read_replicas"])
	require.IsType(t, []any{}, body["maintenances"])
	require.Equal(t, "ready", body["status"])
	require.NotEmpty(t, body["created_at"])

	// Verify GET returns the same enriched fields.
	status, got := testutil.DoGet(t, ts, "/rdb/v1/regions/fr-par/instances/"+body["id"].(string))
	require.Equal(t, 200, status)
	require.NotNil(t, got["volume"])
	require.NotNil(t, got["backup_schedule"])
	require.NotNil(t, got["encryption"])
}

func TestRDBCertificateEndpoint(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name": "db", "engine": "PostgreSQL-15",
	})

	status, body := testutil.DoGet(t, ts,
		"/rdb/v1/regions/fr-par/instances/"+inst["id"].(string)+"/certificate",
	)
	require.Equal(t, 200, status)
	cert := body["certificate"].(map[string]any)
	content := cert["content"].(string)
	require.Contains(t, content, "BEGIN CERTIFICATE")
	require.Contains(t, content, "END CERTIFICATE")
}

func TestRDBACLsSetAndList(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name": "db", "engine": "PostgreSQL-15",
	})
	instID := inst["id"].(string)

	// List ACLs on fresh instance — should be empty.
	status, body := testutil.DoGet(t, ts,
		"/rdb/v1/regions/fr-par/instances/"+instID+"/acls",
	)
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])

	// Set ACLs.
	rules := []any{
		map[string]any{"ip": "0.0.0.0/0", "description": "allow all"},
	}
	status, body = doPut(t, ts,
		"/rdb/v1/regions/fr-par/instances/"+instID+"/acls",
		map[string]any{"rules": rules},
	)
	require.Equal(t, 200, status)
	gotRules := body["rules"].([]any)
	require.Len(t, gotRules, 1)
	require.Equal(t, "0.0.0.0/0", gotRules[0].(map[string]any)["ip"])
}

func TestRDBPrivilegesSetAndList(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name": "db", "engine": "PostgreSQL-15",
	})
	instID := inst["id"].(string)

	// List privileges on fresh instance.
	status, body := testutil.DoGet(t, ts,
		"/rdb/v1/regions/fr-par/instances/"+instID+"/privileges",
	)
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])

	// Set privileges.
	privs := []any{
		map[string]any{"database_name": "appdb", "user_name": "admin", "permission": "all"},
	}
	status, body = doPut(t, ts,
		"/rdb/v1/regions/fr-par/instances/"+instID+"/privileges",
		map[string]any{"privileges": privs},
	)
	require.Equal(t, 200, status)
	gotPrivs := body["privileges"].([]any)
	require.Len(t, gotPrivs, 1)
	require.Equal(t, float64(1), body["total_count"])
}

func TestRDBInstanceDeleteRejectsWhenChildrenExist(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name": "db", "engine": "PostgreSQL-15",
	})
	instID := inst["id"].(string)

	testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID+"/databases", map[string]any{"name": "db1"})
	testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID+"/users", map[string]any{"name": "user1"})

	// Delete with children → 409.
	status := testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID)
	require.Equal(t, 409, status)

	// Delete children first, then instance succeeds.
	testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID+"/databases/db1")
	testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID+"/users/user1")
	status = testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID)
	require.Equal(t, 204, status)

	// Instance should be gone.
	status, _ = testutil.DoGet(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID)
	require.Equal(t, 404, status)
}

func TestLBBackendHasHealthCheckAndTimeouts(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	status, be := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/backends", map[string]any{
		"name": "be", "lb_id": lbID, "forward_port": float64(80),
	})
	require.Equal(t, 200, status)

	// Timeout defaults.
	require.Equal(t, "5m", be["timeout_server"])
	require.Equal(t, "5s", be["timeout_connect"])
	require.Equal(t, "15m", be["timeout_tunnel"])
	require.Equal(t, "0s", be["timeout_queue"])
	require.Equal(t, "none", be["on_marked_down_action"])

	// Health check.
	hc := be["health_check"].(map[string]any)
	require.Equal(t, float64(80), hc["port"])
	require.Equal(t, "60s", hc["check_delay"])
	require.Equal(t, "30s", hc["check_timeout"])
	require.Equal(t, float64(3), hc["check_max_retries"])
	require.NotNil(t, hc["tcp_config"])
}

func TestLBBackendHasLBObject(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	status, be := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/backends", map[string]any{
		"name": "be", "lb_id": lbID,
	})
	require.Equal(t, 200, status)

	lbObj := be["lb"].(map[string]any)
	require.Equal(t, lbID, lbObj["id"])
}

func TestLBFrontendHasLBAndBackendObjects(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	_, be := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/backends", map[string]any{
		"name": "be", "lb_id": lbID,
	})
	beID := be["id"].(string)

	status, fe := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/frontends", map[string]any{
		"name": "http", "lb_id": lbID, "backend_id": beID,
	})
	require.Equal(t, 200, status)

	lbObj := fe["lb"].(map[string]any)
	require.Equal(t, lbID, lbObj["id"])

	beObj := fe["backend"].(map[string]any)
	require.Equal(t, beID, beObj["id"])
}

func TestLBBackendCreateViaNestedRoute(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	// Use nested route: /lbs/{lb_id}/backends
	status, be := testutil.DoCreate(t, ts,
		"/lb/v1/zones/fr-par-1/lbs/"+lbID+"/backends",
		map[string]any{"name": "be-nested"},
	)
	require.Equal(t, 200, status)
	require.NotEmpty(t, be["id"])

	// The lb_id should be set from the URL, and lb object present.
	lbObj := be["lb"].(map[string]any)
	require.Equal(t, lbID, lbObj["id"])
}

func TestLBFrontendCreateViaNestedRoute(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	status, fe := testutil.DoCreate(t, ts,
		"/lb/v1/zones/fr-par-1/lbs/"+lbID+"/frontends",
		map[string]any{"name": "fe-nested"},
	)
	require.Equal(t, 200, status)
	require.NotEmpty(t, fe["id"])

	lbObj := fe["lb"].(map[string]any)
	require.Equal(t, lbID, lbObj["id"])
}

func TestLBPrivateNetworkStatusReady(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})

	status, attachment := testutil.DoCreate(t, ts,
		"/lb/v1/zones/fr-par-1/lbs/"+lbID+"/private-networks",
		map[string]any{"private_network_id": pn["id"]},
	)
	require.Equal(t, 200, status)
	// Provider polls until status is "ready" — must not be "active".
	require.Equal(t, "ready", attachment["status"])
}

func TestLBPrivateNetworkListIncludesLBObject(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})
	testutil.DoCreate(t, ts,
		"/lb/v1/zones/fr-par-1/lbs/"+lbID+"/private-networks",
		map[string]any{"private_network_id": pn["id"]},
	)

	status, body := testutil.DoList(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/private-networks")
	require.Equal(t, 200, status)
	items := body["private_network"].([]any)
	require.Len(t, items, 1)

	// Provider accesses pn.LB.Zone — the LB object must be present.
	item := items[0].(map[string]any)
	lbObj := item["lb"].(map[string]any)
	require.Equal(t, lbID, lbObj["id"])
}

func TestUpdateFrontendEndpoint(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	_, fe := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/frontends", map[string]any{
		"name": "http", "lb_id": lbID, "inbound_port": float64(80),
	})
	feID := fe["id"].(string)

	status, updated := doPut(t, ts,
		"/lb/v1/zones/fr-par-1/frontends/"+feID,
		map[string]any{"inbound_port": float64(443)},
	)
	require.Equal(t, 200, status)
	require.Equal(t, float64(443), updated["inbound_port"])
	require.Equal(t, "http", updated["name"])
}

func TestUpdateBackendEndpoint(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	_, be := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/backends", map[string]any{
		"name": "be", "lb_id": lbID, "forward_port": float64(80),
	})
	beID := be["id"].(string)

	status, updated := doPut(t, ts,
		"/lb/v1/zones/fr-par-1/backends/"+beID,
		map[string]any{"forward_port": float64(8080)},
	)
	require.Equal(t, 200, status)
	require.Equal(t, float64(8080), updated["forward_port"])
	require.Equal(t, "be", updated["name"])
}

func TestFrontendACLsEndpointReturnsEmpty(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	_, fe := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/frontends", map[string]any{
		"name": "http", "lb_id": lbID,
	})
	feID := fe["id"].(string)

	status, body := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+feID+"/acls")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])
	require.Len(t, body["acls"].([]any), 0)
}

func TestLBDeleteRejectsWhenChildrenExist(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	_, fe := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/frontends", map[string]any{
		"name": "fe", "lb_id": lbID,
	})
	_, be := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/backends", map[string]any{
		"name": "be", "lb_id": lbID,
	})

	// Delete with children → 409.
	status := testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID)
	require.Equal(t, 409, status)

	// Delete children first, then LB succeeds.
	testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+fe["id"].(string))
	testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/backends/"+be["id"].(string))
	status = testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID)
	require.Equal(t, 204, status)

	status, _ = testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID)
	require.Equal(t, 404, status)
}

func TestIPAMIPAddressIsString(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})
	_, srv := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "s"})
	serverID := resourceID(srv)

	_, nic := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics",
		map[string]any{"private_network_id": pn["id"]},
	)
	nicID := resourceID(nic)

	status, body := testutil.DoGet(t, ts,
		"/ipam/v1/regions/fr-par/ips?resource_id="+nicID+"&resource_type=instance_private_nic",
	)
	require.Equal(t, 200, status)
	ips := body["ips"].([]any)
	require.Len(t, ips, 1)
	ip := ips[0].(map[string]any)
	// address must be a flat string, not a nested object.
	addr, ok := ip["address"].(string)
	require.True(t, ok, "IPAM address must be a string, got %T", ip["address"])
	require.NotEmpty(t, addr)
}

func TestPrivateNICHasPrivateIPsAndState(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})
	_, srv := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "s"})
	serverID := resourceID(srv)

	status, nic := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics",
		map[string]any{"private_network_id": pn["id"]},
	)
	require.Equal(t, 200, status)
	nicBody := unwrapInstanceResource(nic)

	require.Equal(t, "available", nicBody["state"])
	privIPs := nicBody["private_ips"].([]any)
	require.Len(t, privIPs, 1)
	pip := privIPs[0].(map[string]any)
	require.NotEmpty(t, pip["id"])
	require.NotEmpty(t, pip["address"])
}

func TestPrivateNetworkSubnetsAreObjects(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	status, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})
	require.Equal(t, 200, status)

	subnets := pn["subnets"].([]any)
	require.Len(t, subnets, 1)
	sub := subnets[0].(map[string]any)
	require.NotEmpty(t, sub["id"])
	require.NotEmpty(t, sub["subnet"])
	require.NotEmpty(t, sub["created_at"])
}

func TestVPCv2RoutesWork(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create via v2 API.
	status, vpc := testutil.DoCreate(t, ts, "/vpc/v2/regions/fr-par/vpcs", map[string]any{"name": "v2-vpc"})
	require.Equal(t, 200, status)
	vpcID := vpc["id"].(string)

	status, pn := testutil.DoCreate(t, ts, "/vpc/v2/regions/fr-par/private-networks", map[string]any{
		"name": "v2-pn", "vpc_id": vpcID,
	})
	require.Equal(t, 200, status)
	pnID := pn["id"].(string)

	// Get via v2.
	status, _ = testutil.DoGet(t, ts, "/vpc/v2/regions/fr-par/vpcs/"+vpcID)
	require.Equal(t, 200, status)

	status, _ = testutil.DoGet(t, ts, "/vpc/v2/regions/fr-par/private-networks/"+pnID)
	require.Equal(t, 200, status)

	// List via v2.
	status, body := testutil.DoList(t, ts, "/vpc/v2/regions/fr-par/vpcs")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), body["total_count"])

	// Delete via v2.
	status = testutil.DoDelete(t, ts, "/vpc/v2/regions/fr-par/private-networks/"+pnID)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/vpc/v2/regions/fr-par/vpcs/"+vpcID)
	require.Equal(t, 204, status)
}

// --- Gap coverage: LB IP, DNS, Update handlers, attach enrichment ---

func TestLBIPLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create.
	status, ip := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/ips", map[string]any{})
	require.Equal(t, 200, status)
	ipID := ip["id"].(string)
	require.NotEmpty(t, ipID)
	require.NotEmpty(t, ip["ip_address"])
	require.Equal(t, "fr-par-1", ip["zone"])
	require.Equal(t, "ready", ip["status"])
	require.Equal(t, "fr-par", ip["region"])

	// Get.
	status, got := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/ips/"+ipID)
	require.Equal(t, 200, status)
	require.Equal(t, ipID, got["id"])

	// List.
	status, list := testutil.DoList(t, ts, "/lb/v1/zones/fr-par-1/ips")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), list["total_count"])

	// Delete.
	status = testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/ips/"+ipID)
	require.Equal(t, 204, status)

	// Confirm gone.
	status, _ = testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/ips/"+ipID)
	require.Equal(t, 404, status)
}

func TestDNSZoneListReturnsZones(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// List with domain filter.
	status, body := testutil.DoGet(t, ts, "/domain/v2beta1/dns-zones?domain=example.com")
	require.Equal(t, 200, status)
	zones := body["dns_zones"].([]any)
	require.GreaterOrEqual(t, len(zones), 1)
	zone := zones[0].(map[string]any)
	require.Equal(t, "example.com", zone["domain"])
	require.Equal(t, "active", zone["status"])

	// List with dns_zone filter adds subdomain zone.
	status, body = testutil.DoGet(t, ts, "/domain/v2beta1/dns-zones?domain=example.com&dns_zone=app.example.com")
	require.Equal(t, 200, status)
	zones = body["dns_zones"].([]any)
	require.Len(t, zones, 2)
	sub := zones[1].(map[string]any)
	require.Equal(t, "app", sub["subdomain"])
	require.Equal(t, "example.com", sub["domain"])
}

func TestDNSRecordPatchAndList(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Initially empty.
	status, body := testutil.DoGet(t, ts, "/domain/v2beta1/dns-zones/example.com/records")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), body["total_count"])

	// Add a record via PATCH.
	status, body = doPatch(t, ts, "/domain/v2beta1/dns-zones/example.com/records", map[string]any{
		"changes": []any{
			map[string]any{
				"add": map[string]any{
					"records": []any{
						map[string]any{"name": "app", "type": "A", "data": "1.2.3.4", "ttl": 300},
					},
				},
			},
		},
	})
	require.Equal(t, 200, status)
	records := body["records"].([]any)
	require.Len(t, records, 1)
	rec := records[0].(map[string]any)
	require.Equal(t, "app", rec["name"])
	require.Equal(t, "A", rec["type"])
	require.NotEmpty(t, rec["id"])

	// List confirms the record.
	status, body = testutil.DoGet(t, ts, "/domain/v2beta1/dns-zones/example.com/records")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), body["total_count"])

	// Delete via PATCH.
	recID := rec["id"].(string)
	status, body = doPatch(t, ts, "/domain/v2beta1/dns-zones/example.com/records", map[string]any{
		"changes": []any{
			map[string]any{
				"delete": map[string]any{"id": recID},
			},
		},
	})
	require.Equal(t, 200, status)
	require.Len(t, body["records"].([]any), 0)
}

func TestUpdateRDBInstance(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name": "db", "engine": "PostgreSQL-15", "node_type": "db-dev-s",
	})
	instID := inst["id"].(string)

	status, updated := doPatch(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID, map[string]any{
		"name": "db-renamed", "node_type": "db-dev-m",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "db-renamed", updated["name"])
	require.Equal(t, "db-dev-m", updated["node_type"])
	// Original fields preserved.
	require.Equal(t, "PostgreSQL-15", updated["engine"])
	require.Equal(t, instID, updated["id"])
}

func TestUpdateRDBInstancePersists(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name": "persist-db", "engine": "PostgreSQL-15", "node_type": "db-dev-s",
	})
	instID := inst["id"].(string)

	// Update the name.
	status, _ := doPatch(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID, map[string]any{
		"name": "persist-db-renamed",
	})
	require.Equal(t, 200, status)

	// Re-read from the store — the change must be persisted.
	status, got := testutil.DoGet(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID)
	require.Equal(t, 200, status)
	require.Equal(t, "persist-db-renamed", got["name"])
	require.Equal(t, "PostgreSQL-15", got["engine"])
}

func TestUpdateLB(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	status, updated := doPatch(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID, map[string]any{
		"name": "lb-renamed", "description": "updated",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "lb-renamed", updated["name"])
	require.Equal(t, "updated", updated["description"])
	require.Equal(t, lbID, updated["id"])
}

func TestAttachLBPrivateNetworkResponseHasLBObject(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})

	status, attachment := testutil.DoCreate(t, ts,
		"/lb/v1/zones/fr-par-1/lbs/"+lbID+"/private-networks",
		map[string]any{"private_network_id": pn["id"]},
	)
	require.Equal(t, 200, status)
	// Provider accesses pn.LB.ID after attach.
	lbObj := attachment["lb"].(map[string]any)
	require.Equal(t, lbID, lbObj["id"])
}

func TestAttachLBPrivateNetworkViaAltRoute(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})

	// Use the /attach-private-network alias.
	status, attachment := testutil.DoCreate(t, ts,
		"/lb/v1/zones/fr-par-1/lbs/"+lbID+"/attach-private-network",
		map[string]any{"private_network_id": pn["id"]},
	)
	require.Equal(t, 200, status)
	require.Equal(t, "ready", attachment["status"])
	lbObj := attachment["lb"].(map[string]any)
	require.Equal(t, lbID, lbObj["id"])
}

// TestFullHappyPath exercises the complete flow that infrafactory/tofu performs:
// create VPC → private network → servers → NICs → security groups → IPs →
// LB → backends → frontends → LB private network → RDB instance → databases →
// users → ACLs → privileges → DNS records → IPAM queries →
// then tear everything down in reverse order.
func TestFullHappyPath(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// === Phase 1: VPC & Networking ===

	status, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "main-vpc"})
	require.Equal(t, 200, status)
	vpcID := vpc["id"].(string)

	status, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "app-net", "vpc_id": vpcID,
	})
	require.Equal(t, 200, status)
	pnID := pn["id"].(string)
	// Subnets must be objects with id/subnet fields.
	subnets := pn["subnets"].([]any)
	require.Len(t, subnets, 1)
	require.NotEmpty(t, subnets[0].(map[string]any)["id"])

	// === Phase 2: Instance (Server + SG + IP + NIC) ===

	status, sg := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/security_groups", map[string]any{
		"name": "web-sg", "inbound_default_policy": "drop",
	})
	require.Equal(t, 200, status)
	sgID := resourceID(sg)

	// Set SG rules.
	status, _ = doPut(t, ts, "/instance/v1/zones/fr-par-1/security_groups/"+sgID+"/rules", map[string]any{
		"rules": []any{
			map[string]any{"action": "accept", "protocol": "TCP", "dest_port_from": 80, "direction": "inbound"},
			map[string]any{"action": "accept", "protocol": "TCP", "dest_port_from": 443, "direction": "inbound"},
		},
	})
	require.Equal(t, 200, status)

	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{
		"name": "web-1", "commercial_type": "DEV1-S", "image": "ubuntu_noble", "security_group": sgID,
	})
	require.Equal(t, 200, status)
	serverID := resourceID(server)

	status, ip := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/ips", map[string]any{
		"server_id": serverID,
	})
	require.Equal(t, 200, status)
	ipID := resourceID(ip)

	status, nic := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics", map[string]any{
		"private_network_id": pnID,
	})
	require.Equal(t, 200, status)
	nicBody := unwrapInstanceResource(nic)
	nicID := nicBody["id"].(string)
	require.Equal(t, "available", nicBody["state"])
	require.Len(t, nicBody["private_ips"].([]any), 1)

	// Query IPAM for NIC's private IP — must be a string address.
	status, ipam := testutil.DoGet(t, ts,
		"/ipam/v1/regions/fr-par/ips?resource_id="+nicID+"&resource_type=instance_private_nic",
	)
	require.Equal(t, 200, status)
	ipamIPs := ipam["ips"].([]any)
	require.Len(t, ipamIPs, 1)
	_, addrIsString := ipamIPs[0].(map[string]any)["address"].(string)
	require.True(t, addrIsString, "IPAM address must be a string")

	// === Phase 3: Load Balancer ===

	status, lbIP := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/ips", map[string]any{})
	require.Equal(t, 200, status)
	lbIPID := lbIP["id"].(string)

	status, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{
		"name": "web-lb", "ip_id": lbIPID,
	})
	require.Equal(t, 200, status)
	lbID := lb["id"].(string)

	// Create backend via nested route.
	status, be := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/backends", map[string]any{
		"name": "http-be", "forward_port": float64(80), "forward_protocol": "http",
	})
	require.Equal(t, 200, status)
	beID := be["id"].(string)
	require.NotNil(t, be["lb"])
	require.Equal(t, lbID, be["lb"].(map[string]any)["id"])
	require.NotNil(t, be["health_check"])
	require.Equal(t, "5m", be["timeout_server"])

	// Create frontend via nested route.
	status, fe := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/frontends", map[string]any{
		"name": "http-fe", "inbound_port": float64(80), "backend_id": beID,
	})
	require.Equal(t, 200, status)
	feID := fe["id"].(string)
	require.Equal(t, lbID, fe["lb"].(map[string]any)["id"])
	require.Equal(t, beID, fe["backend"].(map[string]any)["id"])

	// Update frontend.
	status, feUpdated := doPut(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+feID, map[string]any{
		"inbound_port": float64(443),
	})
	require.Equal(t, 200, status)
	require.Equal(t, float64(443), feUpdated["inbound_port"])

	// Frontend ACLs endpoint.
	status, acls := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+feID+"/acls")
	require.Equal(t, 200, status)
	require.Equal(t, float64(0), acls["total_count"])

	// Attach private network to LB.
	status, lbPN := testutil.DoCreate(t, ts,
		"/lb/v1/zones/fr-par-1/lbs/"+lbID+"/private-networks",
		map[string]any{"private_network_id": pnID},
	)
	require.Equal(t, 200, status)
	require.Equal(t, "ready", lbPN["status"])
	require.Equal(t, lbID, lbPN["lb"].(map[string]any)["id"])

	// List LB private networks — must include LB object.
	status, lbPNList := testutil.DoList(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/private-networks")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), lbPNList["total_count"])
	lbPNItem := lbPNList["private_network"].([]any)[0].(map[string]any)
	require.Equal(t, lbID, lbPNItem["lb"].(map[string]any)["id"])

	// === Phase 4: RDB ===

	status, rdb := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name": "app-db", "engine": "PostgreSQL-15", "node_type": "db-dev-s",
		"init_endpoints": []any{
			map[string]any{"private_network": map[string]any{"id": pnID}},
		},
	})
	require.Equal(t, 200, status)
	rdbID := rdb["id"].(string)
	// All TF provider required fields present.
	require.NotNil(t, rdb["volume"])
	require.NotNil(t, rdb["backup_schedule"])
	require.NotNil(t, rdb["encryption"])
	require.NotNil(t, rdb["logs_policy"])
	require.Equal(t, "ready", rdb["status"])
	// Endpoint should reference the private network.
	endpoints := rdb["endpoints"].([]any)
	require.Len(t, endpoints, 1)
	require.Equal(t, pnID, endpoints[0].(map[string]any)["private_network"].(map[string]any)["id"])

	// Certificate.
	status, cert := testutil.DoGet(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/certificate")
	require.Equal(t, 200, status)
	require.Contains(t, cert["certificate"].(map[string]any)["content"].(string), "BEGIN CERTIFICATE")

	// Create database.
	status, db := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/databases", map[string]any{
		"name": "appdb",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "appdb", db["name"])

	// Create user.
	status, user := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/users", map[string]any{
		"name": "admin", "password": "secret123",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "admin", user["name"])

	// Set ACLs.
	status, aclResp := doPut(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/acls", map[string]any{
		"rules": []any{map[string]any{"ip": "0.0.0.0/0"}},
	})
	require.Equal(t, 200, status)
	require.Len(t, aclResp["rules"].([]any), 1)

	// Set privileges.
	status, privResp := doPut(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/privileges", map[string]any{
		"privileges": []any{map[string]any{
			"database_name": "appdb", "user_name": "admin", "permission": "all",
		}},
	})
	require.Equal(t, 200, status)
	require.Len(t, privResp["privileges"].([]any), 1)

	// Update RDB instance.
	status, rdbUpdated := doPatch(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID, map[string]any{
		"name": "app-db-renamed",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "app-db-renamed", rdbUpdated["name"])
	require.Equal(t, "PostgreSQL-15", rdbUpdated["engine"])

	// === Phase 5: DNS ===

	status, zones := testutil.DoGet(t, ts, "/domain/v2beta1/dns-zones?domain=example.com")
	require.Equal(t, 200, status)
	require.GreaterOrEqual(t, len(zones["dns_zones"].([]any)), 1)

	status, records := doPatch(t, ts, "/domain/v2beta1/dns-zones/example.com/records", map[string]any{
		"changes": []any{
			map[string]any{"add": map[string]any{
				"records": []any{map[string]any{"name": "app", "type": "A", "data": "1.2.3.4", "ttl": 300}},
			}},
		},
	})
	require.Equal(t, 200, status)
	require.Len(t, records["records"].([]any), 1)
	recID := records["records"].([]any)[0].(map[string]any)["id"].(string)

	// === Phase 6: IAM ===

	status, app := testutil.DoCreate(t, ts, "/iam/v1alpha1/applications", map[string]any{"name": "tf-app"})
	require.Equal(t, 200, status)
	appID := app["id"].(string)

	status, apiKey := testutil.DoCreate(t, ts, "/iam/v1alpha1/api-keys", map[string]any{"application_id": appID})
	require.Equal(t, 200, status)
	accessKey := apiKey["access_key"].(string)
	require.NotEmpty(t, apiKey["secret_key"])

	status, policy := testutil.DoCreate(t, ts, "/iam/v1alpha1/policies", map[string]any{
		"name": "admin-policy", "application_id": appID,
	})
	require.Equal(t, 200, status)
	policyID := policy["id"].(string)

	status, sshKey := testutil.DoCreate(t, ts, "/iam/v1alpha1/ssh-keys", map[string]any{
		"name": "deploy-key", "public_key": "ssh-ed25519 AAAA",
	})
	require.Equal(t, 200, status)
	sshKeyID := sshKey["id"].(string)

	// === Phase 7: Verify full state ===

	state := testutil.GetState(t, ts)
	require.Len(t, state["instance"].(map[string]any)["servers"].([]any), 1)
	require.Len(t, state["vpc"].(map[string]any)["vpcs"].([]any), 1)
	require.Len(t, state["lb"].(map[string]any)["lbs"].([]any), 1)
	require.Len(t, state["rdb"].(map[string]any)["instances"].([]any), 1)

	// === Phase 8: Tear down (reverse order, matching tofu destroy) ===

	// IAM.
	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/ssh-keys/"+sshKeyID)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/policies/"+policyID)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/api-keys/"+accessKey)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/iam/v1alpha1/applications/"+appID)
	require.Equal(t, 204, status)

	// DNS record.
	status, _ = doPatch(t, ts, "/domain/v2beta1/dns-zones/example.com/records", map[string]any{
		"changes": []any{map[string]any{"delete": map[string]any{"id": recID}}},
	})
	require.Equal(t, 200, status)

	// RDB: delete children first (privileges, databases, users), then instance.
	doPut(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/privileges", map[string]any{
		"privileges": []any{},
	})
	testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/databases/appdb")
	testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID+"/users/admin")
	status = testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+rdbID)
	require.Equal(t, 204, status)

	// LB: delete frontends and backends (provider does this explicitly),
	// then LB (private networks cascade automatically).
	testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+feID)
	testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/backends/"+beID)
	status = testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID)
	require.Equal(t, 204, status)

	status = testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/ips/"+lbIPID)
	require.Equal(t, 204, status)

	// Instance.
	status = testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/ips/"+ipID)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/security_groups/"+sgID)
	require.Equal(t, 204, status)

	// VPC.
	status = testutil.DoDelete(t, ts, "/vpc/v1/regions/fr-par/private-networks/"+pnID)
	require.Equal(t, 204, status)
	status = testutil.DoDelete(t, ts, "/vpc/v1/regions/fr-par/vpcs/"+vpcID)
	require.Equal(t, 204, status)

	// === Phase 9: Verify everything is gone ===

	state = testutil.GetState(t, ts)
	require.Len(t, state["instance"].(map[string]any)["servers"].([]any), 0)
	require.Len(t, state["vpc"].(map[string]any)["vpcs"].([]any), 0)
	require.Len(t, state["vpc"].(map[string]any)["private_networks"].([]any), 0)
	require.Len(t, state["lb"].(map[string]any)["lbs"].([]any), 0)
	require.Len(t, state["lb"].(map[string]any)["frontends"].([]any), 0)
	require.Len(t, state["lb"].(map[string]any)["backends"].([]any), 0)
	require.Len(t, state["rdb"].(map[string]any)["instances"].([]any), 0)
	require.Len(t, state["rdb"].(map[string]any)["databases"].([]any), 0)
	require.Len(t, state["rdb"].(map[string]any)["users"].([]any), 0)
	require.Len(t, state["rdb"].(map[string]any)["privileges"].([]any), 0)
	require.Len(t, state["iam"].(map[string]any)["applications"].([]any), 0)
}

func TestSetRDBSettingsEndpoint(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create an RDB instance first.
	status, body := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":      "settings-test",
		"engine":    "PostgreSQL-15",
		"node_type": "DB-DEV-S",
	})
	require.Equal(t, 200, status)
	instanceID := body["id"].(string)

	// PUT settings should return 200.
	status, resp := doPut(t, ts, "/rdb/v1/regions/fr-par/instances/"+instanceID+"/settings", map[string]any{
		"settings": []any{
			map[string]any{"name": "effective_cache_size", "value": "1000"},
		},
	})
	require.Equal(t, 200, status)
	settings := resp["settings"].([]any)
	require.Len(t, settings, 1)
}

func TestRDBPrivilegePersistence(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create RDB instance, database, and user.
	status, body := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":      "priv-test",
		"engine":    "PostgreSQL-15",
		"node_type": "DB-DEV-S",
	})
	require.Equal(t, 200, status)
	instanceID := body["id"].(string)

	status, _ = testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+instanceID+"/databases", map[string]any{
		"name": "webapp",
	})
	require.Equal(t, 200, status)

	status, _ = testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+instanceID+"/users", map[string]any{
		"name":     "webapp-user",
		"password": "changeme",
	})
	require.Equal(t, 200, status)

	// Set privileges.
	status, resp := doPut(t, ts, "/rdb/v1/regions/fr-par/instances/"+instanceID+"/privileges", map[string]any{
		"privileges": []any{
			map[string]any{
				"database_name": "webapp",
				"user_name":     "webapp-user",
				"permission":    "all",
			},
		},
	})
	require.Equal(t, 200, status)
	privs := resp["privileges"].([]any)
	require.Len(t, privs, 1)

	// List privileges should return the persisted privilege.
	status, resp = testutil.DoGet(t, ts, "/rdb/v1/regions/fr-par/instances/"+instanceID+"/privileges")
	require.Equal(t, 200, status)
	privs = resp["privileges"].([]any)
	require.Len(t, privs, 1)
	priv := privs[0].(map[string]any)
	require.Equal(t, "webapp", priv["database_name"])
	require.Equal(t, "webapp-user", priv["user_name"])
	require.Equal(t, "all", priv["permission"])

	// Verify privileges appear in mock state.
	state := testutil.GetState(t, ts)
	rdbState := state["rdb"].(map[string]any)
	statePrivs := rdbState["privileges"].([]any)
	require.Len(t, statePrivs, 1)

	// Delete instance with dependents → 409.
	status = testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+instanceID)
	require.Equal(t, 409, status)
}

func TestBlockVolumeEndpoints(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// 1. Create a server (which embeds a volume in its "volumes" field).
	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "block-vol-test"},
	)
	require.Equal(t, 200, status)
	server := unwrapInstanceResource(body)
	serverID := server["id"].(string)

	// 2. Extract the volume ID from the server's volumes map.
	volumes := server["volumes"].(map[string]any)
	root := volumes["0"].(map[string]any)
	volumeID := root["id"].(string)
	require.NotEmpty(t, volumeID)

	// 3. GET the volume via the block API and verify it returns the volume.
	status, body = testutil.DoGet(t, ts, "/block/v1alpha1/zones/fr-par-1/volumes/"+volumeID)
	require.Equal(t, 200, status)
	volume := body["volume"].(map[string]any)
	require.Equal(t, volumeID, volume["id"])

	// 4. DELETE the volume via the block API and verify 204.
	status = testutil.DoDelete(t, ts, "/block/v1alpha1/zones/fr-par-1/volumes/"+volumeID)
	require.Equal(t, 204, status)

	// 5. Terminate the server, then verify the volume returns 404.
	status, resp := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/action",
		map[string]any{"action": "terminate"},
	)
	require.Equal(t, 200, status)
	task := resp["task"].(map[string]any)
	require.Equal(t, "terminate", task["description"])

	// After termination, the server (and its volumes) are gone; GET should 404.
	status, body = testutil.DoGet(t, ts, "/block/v1alpha1/zones/fr-par-1/volumes/"+volumeID)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
}

func TestUpdateCluster(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters", map[string]any{
		"name": "mycluster", "version": "1.28",
	})
	clusterID := cluster["id"].(string)

	status, updated := doPatch(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID, map[string]any{
		"name": "renamed-cluster", "version": "1.29",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "renamed-cluster", updated["name"])
	require.Equal(t, "1.29", updated["version"])
	require.Equal(t, clusterID, updated["id"])
}

func TestUpdatePool(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters", map[string]any{"name": "c"})
	clusterID := cluster["id"].(string)

	_, pool := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID+"/pools", map[string]any{
		"name": "pool", "node_type": "DEV1-M", "size": float64(1),
	})
	poolID := pool["id"].(string)

	status, updated := doPatch(t, ts, "/k8s/v1/regions/fr-par/pools/"+poolID, map[string]any{
		"size": float64(3),
	})
	require.Equal(t, 200, status)
	require.Equal(t, float64(3), updated["size"])
	require.Equal(t, "pool", updated["name"])
	require.Equal(t, poolID, updated["id"])
}

func TestDeleteClusterRejectsWhenPoolsExist(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters", map[string]any{"name": "c"})
	clusterID := cluster["id"].(string)
	_, pool1 := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID+"/pools", map[string]any{"name": "p1"})
	_, pool2 := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID+"/pools", map[string]any{"name": "p2"})

	// Delete cluster with pools still present → 409 Conflict.
	status := testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID)
	require.Equal(t, 409, status)

	// Cluster and pools still exist.
	status, _ = testutil.DoGet(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID)
	require.Equal(t, 200, status)
	status, body := testutil.DoList(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID+"/pools")
	require.Equal(t, 200, status)
	require.Equal(t, float64(2), body["total_count"])

	// Delete pools first, then cluster succeeds.
	testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/pools/"+pool1["id"].(string))
	testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/pools/"+pool2["id"].(string))
	status = testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/clusters/"+clusterID)
	require.Equal(t, 200, status)
}

func TestDeleteLBRejectsWhenDependentsExist(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb1"})
	lbID := lb["id"].(string)
	_, fe := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/frontends", map[string]any{
		"name":         "fe",
		"inbound_port": float64(80),
	})

	// Delete LB with frontend present → 409.
	status := testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID)
	require.Equal(t, 409, status)

	// Delete frontend first, then LB succeeds.
	testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+fe["id"].(string))
	status = testutil.DoDelete(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID)
	require.Equal(t, 204, status)
}

func TestDeleteRDBInstanceRejectsWhenDependentsExist(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{"name": "db"})
	instID := inst["id"].(string)
	testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID+"/databases", map[string]any{"name": "appdb"})

	// Delete instance with database present → 409.
	status := testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID)
	require.Equal(t, 409, status)

	// Instance still exists.
	status, _ = testutil.DoGet(t, ts, "/rdb/v1/regions/fr-par/instances/"+instID)
	require.Equal(t, 200, status)
}

func TestRegistryNamespaceLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create namespace.
	status, ns := testutil.DoCreate(t, ts, "/registry/v1/regions/fr-par/namespaces", map[string]any{
		"name": "my-registry",
	})
	require.Equal(t, 200, status)
	nsID := ns["id"].(string)
	require.NotEmpty(t, nsID)
	require.Equal(t, "my-registry", ns["name"])
	require.Equal(t, "ready", ns["status"])
	require.NotEmpty(t, ns["endpoint"])
	require.NotEmpty(t, ns["created_at"])

	// Get namespace.
	status, got := testutil.DoGet(t, ts, "/registry/v1/regions/fr-par/namespaces/"+nsID)
	require.Equal(t, 200, status)
	require.Equal(t, nsID, got["id"])
	require.Equal(t, "my-registry", got["name"])

	// List namespaces.
	status, list := testutil.DoList(t, ts, "/registry/v1/regions/fr-par/namespaces")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), list["total_count"])
	require.Len(t, list["namespaces"].([]any), 1)

	// Delete namespace.
	status = testutil.DoDelete(t, ts, "/registry/v1/regions/fr-par/namespaces/"+nsID)
	require.Equal(t, 204, status)

	// Confirm deleted.
	status, _ = testutil.DoGet(t, ts, "/registry/v1/regions/fr-par/namespaces/"+nsID)
	require.Equal(t, 404, status)
}

func TestRegistryNamespaceUpdate(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, ns := testutil.DoCreate(t, ts, "/registry/v1/regions/fr-par/namespaces", map[string]any{
		"name": "orig",
	})
	nsID := ns["id"].(string)

	status, updated := doPatch(t, ts, "/registry/v1/regions/fr-par/namespaces/"+nsID, map[string]any{
		"description": "updated description",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "orig", updated["name"])
	require.Equal(t, "updated description", updated["description"])
	require.Equal(t, nsID, updated["id"])
}

func TestRegistryServiceState(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	testutil.DoCreate(t, ts, "/registry/v1/regions/fr-par/namespaces", map[string]any{
		"name": "state-test",
	})

	status, body := testutil.DoGet(t, ts, "/mock/state/registry")
	require.Equal(t, 200, status)
	require.Contains(t, body, "namespaces")
	nsList := body["namespaces"].([]any)
	require.Len(t, nsList, 1)
}

func TestRedisClusterLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create cluster.
	status, cluster := testutil.DoCreate(t, ts, "/redis/v1/zones/fr-par-1/clusters", map[string]any{
		"name": "my-redis", "version": "7.0.12", "node_type": "RED1-MICRO",
	})
	require.Equal(t, 200, status)
	clusterID := cluster["id"].(string)
	require.NotEmpty(t, clusterID)
	require.Equal(t, "my-redis", cluster["name"])
	require.Equal(t, "ready", cluster["status"])
	require.NotEmpty(t, cluster["created_at"])

	// Get cluster.
	status, got := testutil.DoGet(t, ts, "/redis/v1/zones/fr-par-1/clusters/"+clusterID)
	require.Equal(t, 200, status)
	require.Equal(t, clusterID, got["id"])
	require.Equal(t, "my-redis", got["name"])

	// List clusters.
	status, list := testutil.DoList(t, ts, "/redis/v1/zones/fr-par-1/clusters")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), list["total_count"])
	require.Len(t, list["clusters"].([]any), 1)

	// Delete cluster.
	status = testutil.DoDelete(t, ts, "/redis/v1/zones/fr-par-1/clusters/"+clusterID)
	require.Equal(t, 204, status)

	// Confirm deleted.
	status, _ = testutil.DoGet(t, ts, "/redis/v1/zones/fr-par-1/clusters/"+clusterID)
	require.Equal(t, 404, status)
}

func TestRedisClusterUpdate(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, cluster := testutil.DoCreate(t, ts, "/redis/v1/zones/fr-par-1/clusters", map[string]any{
		"name": "redis-orig", "version": "7.0.12", "node_type": "RED1-MICRO",
	})
	clusterID := cluster["id"].(string)

	status, updated := doPatch(t, ts, "/redis/v1/zones/fr-par-1/clusters/"+clusterID, map[string]any{
		"name": "redis-renamed",
	})
	require.Equal(t, 200, status)
	require.Equal(t, "redis-renamed", updated["name"])
	require.Equal(t, clusterID, updated["id"])
}

func TestRedisServiceState(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	testutil.DoCreate(t, ts, "/redis/v1/zones/fr-par-1/clusters", map[string]any{
		"name": "state-redis", "version": "7.0.12", "node_type": "RED1-MICRO",
	})

	status, body := testutil.DoGet(t, ts, "/mock/state/redis")
	require.Equal(t, 200, status)
	require.Contains(t, body, "clusters")
	clusters := body["clusters"].([]any)
	require.Len(t, clusters, 1)
}

func TestRedisClusterHasProviderRequiredFields(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, cluster := testutil.DoCreate(t, ts, "/redis/v1/zones/fr-par-1/clusters", map[string]any{
		"name": "field-check", "version": "7.0.12", "node_type": "RED1-MICRO",
	})

	// Fields the TF provider reads that were previously missing.
	require.Equal(t, "00000000-0000-0000-0000-000000000000", cluster["organization_id"])
	require.Equal(t, "00000000-0000-0000-0000-000000000000", cluster["project_id"])
	require.NotNil(t, cluster["tags"])
	require.NotNil(t, cluster["acl_rules"])
	require.NotNil(t, cluster["endpoints"])
	require.NotNil(t, cluster["public_network"])
	require.NotNil(t, cluster["settings"])
	require.Equal(t, "default", cluster["user_name"])

	// Endpoints should contain at least one entry with a port.
	endpoints := cluster["endpoints"].([]any)
	require.GreaterOrEqual(t, len(endpoints), 1)
	ep0 := endpoints[0].(map[string]any)
	require.Equal(t, float64(6379), ep0["port"])
}

func TestUpdateLBPersists(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)

	status, _ := doPatch(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID, map[string]any{
		"name": "lb-persisted",
	})
	require.Equal(t, 200, status)

	status, got := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID)
	require.Equal(t, 200, status)
	require.Equal(t, "lb-persisted", got["name"])
}

func TestUpdateFrontendPersists(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)
	_, backend := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/backends",
		map[string]any{"name": "be", "forward_port": 80, "forward_protocol": "tcp"})
	backendID := backend["id"].(string)
	_, frontend := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/frontends",
		map[string]any{"name": "fe", "inbound_port": 80, "backend_id": backendID})
	feID := frontend["id"].(string)

	status, _ := doPut(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+feID, map[string]any{
		"name": "fe-renamed", "inbound_port": 443,
	})
	require.Equal(t, 200, status)

	status, got := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+feID)
	require.Equal(t, 200, status)
	require.Equal(t, "fe-renamed", got["name"])
}

func TestUpdateBackendPersists(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb"})
	lbID := lb["id"].(string)
	_, backend := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lbID+"/backends",
		map[string]any{"name": "be", "forward_port": 80, "forward_protocol": "tcp"})
	beID := backend["id"].(string)

	status, _ := doPut(t, ts, "/lb/v1/zones/fr-par-1/backends/"+beID, map[string]any{
		"name": "be-renamed",
	})
	require.Equal(t, 200, status)

	status, got := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/backends/"+beID)
	require.Equal(t, 200, status)
	require.Equal(t, "be-renamed", got["name"])
}

func TestListFrontendsFiltersByLBID(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// Create two LBs with one frontend each.
	_, lb1 := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb1"})
	lb1ID := lb1["id"].(string)
	_, lb2 := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb2"})
	lb2ID := lb2["id"].(string)

	_, be1 := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb1ID+"/backends",
		map[string]any{"name": "be1", "forward_port": 80, "forward_protocol": "tcp"})
	_, be2 := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb2ID+"/backends",
		map[string]any{"name": "be2", "forward_port": 80, "forward_protocol": "tcp"})

	testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb1ID+"/frontends",
		map[string]any{"name": "fe1", "inbound_port": 80, "backend_id": be1["id"]})
	testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb2ID+"/frontends",
		map[string]any{"name": "fe2", "inbound_port": 443, "backend_id": be2["id"]})

	// Nested list for LB1 should only return fe1.
	status, list := testutil.DoList(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb1ID+"/frontends")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), list["total_count"])
	items := list["frontends"].([]any)
	require.Len(t, items, 1)
	fe := items[0].(map[string]any)
	require.Equal(t, "fe1", fe["name"])

	// Global list should return both.
	status, globalList := testutil.DoList(t, ts, "/lb/v1/zones/fr-par-1/frontends")
	require.Equal(t, 200, status)
	require.Equal(t, float64(2), globalList["total_count"])
}

func TestListBackendsFiltersByLBID(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lb1 := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb1"})
	lb1ID := lb1["id"].(string)
	_, lb2 := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb2"})
	lb2ID := lb2["id"].(string)

	testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb1ID+"/backends",
		map[string]any{"name": "be1", "forward_port": 80, "forward_protocol": "tcp"})
	testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb2ID+"/backends",
		map[string]any{"name": "be2", "forward_port": 80, "forward_protocol": "tcp"})

	// Nested list for LB1 should only return be1.
	status, list := testutil.DoList(t, ts, "/lb/v1/zones/fr-par-1/lbs/"+lb1ID+"/backends")
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), list["total_count"])
	items := list["backends"].([]any)
	require.Len(t, items, 1)
	be := items[0].(map[string]any)
	require.Equal(t, "be1", be["name"])
}

func TestRDBCertificateNotFoundForMissingInstance(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, _ := testutil.DoGet(t, ts, "/rdb/v1/regions/fr-par/instances/nonexistent-id/certificate")
	require.Equal(t, 404, status)
}

func TestCreateLBWithMalformedZoneDoesNotPanic(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	// A zone with only one part should not cause an index-out-of-range panic.
	status, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/badzone/lbs", map[string]any{"name": "lb"})
	require.Equal(t, 200, status)
	require.NotEmpty(t, lb["id"])
	// Region should fall back gracefully.
	ips := lb["ip"].([]any)
	ip0 := ips[0].(map[string]any)
	require.Equal(t, "badzone", ip0["region"])
}

func TestUpdateRedisClusterCannotMutateID(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, cluster := testutil.DoCreate(t, ts, "/redis/v1/zones/fr-par-1/clusters", map[string]any{
		"name": "redis-id-test", "version": "7.0.12", "node_type": "RED1-MICRO",
	})
	origID := cluster["id"].(string)

	status, updated := doPatch(t, ts, "/redis/v1/zones/fr-par-1/clusters/"+origID, map[string]any{
		"name": "renamed", "id": "injected-id",
	})
	require.Equal(t, 200, status)
	require.Equal(t, origID, updated["id"], "id should not be overwritten by patch")
	require.Equal(t, "renamed", updated["name"])

	// Confirm via GET that the stored ID is still correct.
	status, got := testutil.DoGet(t, ts, "/redis/v1/zones/fr-par-1/clusters/"+origID)
	require.Equal(t, 200, status)
	require.Equal(t, origID, got["id"])
}

func TestUpdateRegistryNamespaceCannotMutateID(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, ns := testutil.DoCreate(t, ts, "/registry/v1/regions/fr-par/namespaces", map[string]any{
		"name": "reg-id-test",
	})
	origID := ns["id"].(string)

	status, updated := doPatch(t, ts, "/registry/v1/regions/fr-par/namespaces/"+origID, map[string]any{
		"name": "reg-renamed", "id": "injected-id",
	})
	require.Equal(t, 200, status)
	require.Equal(t, origID, updated["id"], "id should not be overwritten by patch")

	status, got := testutil.DoGet(t, ts, "/registry/v1/regions/fr-par/namespaces/"+origID)
	require.Equal(t, 200, status)
	require.Equal(t, origID, got["id"])
}
