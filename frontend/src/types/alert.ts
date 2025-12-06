/**
 * Alert information from Alertmanager
 */
export interface Alert {
  /** Key-value pairs of alert labels */
  labels: Record<string, string>;
  /** Key-value pairs of alert annotations */
  annotations: Record<string, string>;
  /** Time when the alert started firing */
  startsAt?: string;
  /** Time when the alert ended */
  endsAt?: string;
}

/**
 * Condition of a job definition
 */
export interface JobCondition {
  /** Type of condition */
  type: string;
  /** Status of the condition */
  status: string;
  /** Last time the condition transitioned from one status to another */
  lastTransitionTime: string;
  /** Unique, one-word, CamelCase reason for the condition's last transition */
  reason?: string;
  /** Human-readable message indicating details about last transition */
  message?: string;
}

/**
 * Information about a triggered remediation job
 */
export interface JobInfo {
  /** Name of the ConfigMap containing the job definition */
  configMapName: string;
  /** Name of the job */
  jobName: string;
  /** Namespace of the job */
  namespace?: string;
  /** Container image used by the job */
  image: string;
  /** Total number of jobs created from this Operarius */
  executionCount?: number;
  /** Last time a job was created from this Operarius */
  lastExecutionTime?: string;
  /** Name of the last job created */
  lastExecutedJobName?: string;
  /** Latest available observations of an Operarius's state */
  conditions?: JobCondition[];
  /** Job execution status */
  status?: string;
  /** Time when the job started */
  startedAt?: string;
  /** Time when the job completed */
  completedAt?: string;
}

/**
 * Stored alert entry with status and metadata
 */
export interface AlertStoreEntry {
  /** The alert data */
  alert: Alert;
  /** Alert status: firing or resolved */
  status: "firing" | "resolved";
  /** Timestamp when the alert was stored */
  timestamp: string;
  /** Information about the triggered job, if any */
  jobInfo?: JobInfo;
}

/**
 * Hook message received from Alertmanager
 */
export interface HookMessage {
  /** Version of the Alertmanager message */
  version: string;
  /** Key used to group alerts */
  groupKey: string;
  /** Status of the alert group */
  status: "firing" | "resolved";
  /** Name of the receiver that handled the alert */
  receiver: string;
  /** Labels common to all alerts in the group */
  groupLabels: Record<string, string>;
  /** Labels common across all alerts */
  commonLabels: Record<string, string>;
  /** Annotations common across all alerts */
  commonAnnotations: Record<string, string>;
  /** External URL to the Alertmanager */
  externalURL: string;
  /** List of alerts in the group */
  alerts: Alert[];
}
