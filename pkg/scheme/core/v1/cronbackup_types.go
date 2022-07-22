package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=false

type CronBackup struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CronBackupSpec `json:"spec"`
	// +optional
	Status CronBackupStatus `json:"status,omitempty"`
}

// CronBackupSpec defines the desired state of CronBackup
type CronBackupSpec struct {
	// the cluster to which the cronBackup belongs
	ClusterName string `json:"clusterName,omitempty"`
	// the schedule in cron format
	Schedule string `json:"schedule,omitempty"`
	// maximum number of reserved backups
	MaxBackupNum int `json:"maxBackupNum,omitempty"`
	// specific run time
	RunAt *metav1.Time `json:"runAt,omitempty"`
}

// CronBackupStatus defines the status of cronBackup
type CronBackupStatus struct {
	// next create backup time
	NextScheduleTime *metav1.Time `json:"NextScheduleTime,omitempty"`
	// last create backup time
	LastScheduleTime *metav1.Time `json:"LastScheduleTime,omitempty"`
	// last successfully create backup time
	LastSuccessfulTime *metav1.Time `json:"LastSuccessfulTime,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// CronBackupList contains a list of CronBackup

type CronBackupList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronBackup `json:"items"`
}
