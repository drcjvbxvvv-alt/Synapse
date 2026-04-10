package handlers

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ─── Snapshot Helpers ────────────────────────────────────────────────────────

func buildSnapshotSpec(pvcName, snapshotClassName string) map[string]interface{} {
	spec := map[string]interface{}{
		"source": map[string]interface{}{
			"persistentVolumeClaimName": pvcName,
		},
	}
	if snapshotClassName != "" {
		spec["volumeSnapshotClassName"] = snapshotClassName
	}
	return spec
}

func snapshotToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	source, _ := spec["source"].(map[string]interface{})
	sourcePVC, _ := source["persistentVolumeClaimName"].(string)
	snapshotClassName, _ := spec["volumeSnapshotClassName"].(string)

	readyToUse, _ := status["readyToUse"].(bool)
	restoreSize, _ := status["restoreSize"].(string)
	boundContentName, _ := status["boundVolumeSnapshotContentName"].(string)

	// error message if any
	errMsg := ""
	if errObj, ok := status["error"].(map[string]interface{}); ok {
		errMsg, _ = errObj["message"].(string)
	}

	return map[string]interface{}{
		"name":              obj.GetName(),
		"namespace":         obj.GetNamespace(),
		"sourcePVC":         sourcePVC,
		"snapshotClassName": snapshotClassName,
		"readyToUse":        readyToUse,
		"restoreSize":       restoreSize,
		"boundContentName":  boundContentName,
		"error":             errMsg,
		"createdAt":         obj.GetCreationTimestamp().Time,
	}
}

// ─── Velero info converters ──────────────────────────────────────────────────

func veleroBackupToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	phase, _ := status["phase"].(string)
	startTimestamp, _ := status["startTimestamp"].(string)
	completionTimestamp, _ := status["completionTimestamp"].(string)
	expiration, _ := status["expiration"].(string)
	progress, _ := status["progress"].(map[string]interface{})

	includedNS, _ := spec["includedNamespaces"].([]interface{})
	storageLocation, _ := spec["storageLocation"].(string)
	ttl, _ := spec["ttl"].(string)

	return map[string]interface{}{
		"name":                obj.GetName(),
		"namespace":           obj.GetNamespace(),
		"phase":               phase,
		"includedNamespaces":  includedNS,
		"storageLocation":     storageLocation,
		"ttl":                 ttl,
		"startTimestamp":      startTimestamp,
		"completionTimestamp": completionTimestamp,
		"expiration":          expiration,
		"progress":            progress,
		"createdAt":           obj.GetCreationTimestamp().Time,
	}
}

func veleroRestoreToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	backupName, _ := spec["backupName"].(string)
	phase, _ := status["phase"].(string)
	warnings, _ := status["warnings"].(int64)
	errors, _ := status["errors"].(int64)

	return map[string]interface{}{
		"name":       obj.GetName(),
		"namespace":  obj.GetNamespace(),
		"backupName": backupName,
		"phase":      phase,
		"warnings":   warnings,
		"errors":     errors,
		"createdAt":  obj.GetCreationTimestamp().Time,
	}
}

func veleroScheduleToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	schedule, _ := spec["schedule"].(string)
	paused, _ := spec["paused"].(bool)
	lastBackup, _ := status["lastBackup"].(string)
	phase, _ := status["phase"].(string)

	tmpl, _ := spec["template"].(map[string]interface{})
	storageLocation, _ := tmpl["storageLocation"].(string)
	ttl, _ := tmpl["ttl"].(string)

	return map[string]interface{}{
		"name":            obj.GetName(),
		"namespace":       obj.GetNamespace(),
		"schedule":        schedule,
		"paused":          paused,
		"phase":           phase,
		"lastBackup":      lastBackup,
		"storageLocation": storageLocation,
		"ttl":             ttl,
		"createdAt":       obj.GetCreationTimestamp().Time,
	}
}
