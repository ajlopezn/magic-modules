package google

import (
	"fmt"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/api/cloudresourcemanager/v1"
)

var StorageBucketIamSchema = map[string]*schema.Schema{
	"bucket": {
		Type:             schema.TypeString,
		Required:         true,
		ForceNew:         true,
		DiffSuppressFunc: StorageBucketDiffSuppress,
	},
}

func StorageBucketDiffSuppress(_, old, new string, _ *schema.ResourceData) bool {
	return compareResourceNames("", old, new, nil)
}

type StorageBucketIamUpdater struct {
	bucket string
	d      TerraformResourceData
	Config *Config
}

func StorageBucketIamUpdaterProducer(d TerraformResourceData, config *Config) (ResourceIamUpdater, error) {
	values := make(map[string]string)

	if v, ok := d.GetOk("bucket"); ok {
		values["bucket"] = v.(string)
	}

	// We may have gotten either a long or short name, so attempt to parse long name if possible
	m, err := getImportIdQualifiers([]string{"b/(?P<bucket>[^/]+)", "(?P<bucket>[^/]+)"}, d, config, d.Get("bucket").(string))
	if err != nil {
		return nil, err
	}

	for k, v := range m {
		values[k] = v
	}

	u := &StorageBucketIamUpdater{
		bucket: values["bucket"],
		d:      d,
		Config: config,
	}

	if err := d.Set("bucket", u.GetResourceId()); err != nil {
		return nil, fmt.Errorf("Error setting bucket: %s", err)
	}

	return u, nil
}

func StorageBucketIdParseFunc(d *schema.ResourceData, config *Config) error {
	values := make(map[string]string)

	m, err := getImportIdQualifiers([]string{"b/(?P<bucket>[^/]+)", "(?P<bucket>[^/]+)"}, d, config, d.Id())
	if err != nil {
		return err
	}

	for k, v := range m {
		values[k] = v
	}

	u := &StorageBucketIamUpdater{
		bucket: values["bucket"],
		d:      d,
		Config: config,
	}
	if err := d.Set("bucket", u.GetResourceId()); err != nil {
		return fmt.Errorf("Error setting bucket: %s", err)
	}
	d.SetId(u.GetResourceId())
	return nil
}

func (u *StorageBucketIamUpdater) GetResourceIamPolicy() (*cloudresourcemanager.Policy, error) {
	url, err := u.qualifyBucketUrl("iam")
	if err != nil {
		return nil, err
	}

	var obj map[string]interface{}
	url, err = AddQueryParams(url, map[string]string{"optionsRequestedPolicyVersion": fmt.Sprintf("%d", IamPolicyVersion)})
	if err != nil {
		return nil, err
	}

	userAgent, err := generateUserAgentString(u.d, u.Config.UserAgent)
	if err != nil {
		return nil, err
	}

	policy, err := SendRequest(u.Config, "GET", "", url, userAgent, obj)
	if err != nil {
		return nil, errwrap.Wrapf(fmt.Sprintf("Error retrieving IAM policy for %s: {{err}}", u.DescribeResource()), err)
	}

	out := &cloudresourcemanager.Policy{}
	err = Convert(policy, out)
	if err != nil {
		return nil, errwrap.Wrapf("Cannot convert a policy to a resource manager policy: {{err}}", err)
	}

	return out, nil
}

func (u *StorageBucketIamUpdater) SetResourceIamPolicy(policy *cloudresourcemanager.Policy) error {
	json, err := ConvertToMap(policy)
	if err != nil {
		return err
	}

	obj := json

	url, err := u.qualifyBucketUrl("iam")
	if err != nil {
		return err
	}

	userAgent, err := generateUserAgentString(u.d, u.Config.UserAgent)
	if err != nil {
		return err
	}

	_, err = SendRequestWithTimeout(u.Config, "PUT", "", url, userAgent, obj, u.d.Timeout(schema.TimeoutCreate))
	if err != nil {
		return errwrap.Wrapf(fmt.Sprintf("Error setting IAM policy for %s: {{err}}", u.DescribeResource()), err)
	}

	return nil
}

func (u *StorageBucketIamUpdater) qualifyBucketUrl(methodIdentifier string) (string, error) {
	urlTemplate := fmt.Sprintf("{{StorageBasePath}}%s/%s", fmt.Sprintf("b/%s", u.bucket), methodIdentifier)
	url, err := ReplaceVars(u.d, u.Config, urlTemplate)
	if err != nil {
		return "", err
	}
	return url, nil
}

func (u *StorageBucketIamUpdater) GetResourceId() string {
	return fmt.Sprintf("b/%s", u.bucket)
}

func (u *StorageBucketIamUpdater) GetMutexKey() string {
	return fmt.Sprintf("iam-storage-bucket-%s", u.GetResourceId())
}

func (u *StorageBucketIamUpdater) DescribeResource() string {
	return fmt.Sprintf("storage bucket %q", u.GetResourceId())
}
