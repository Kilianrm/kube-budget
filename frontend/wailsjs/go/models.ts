export namespace cluster {
	
	export class ClusterInfo {
	    context: string;
	    server: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.server = source["server"];
	        this.version = source["version"];
	    }
	}
	export class ResourceValues {
	    cpuMilli: number;
	    memoryBytes: number;
	    storageBytes: number;
	    gpuUnits: number;
	
	    static createFrom(source: any = {}) {
	        return new ResourceValues(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuMilli = source["cpuMilli"];
	        this.memoryBytes = source["memoryBytes"];
	        this.storageBytes = source["storageBytes"];
	        this.gpuUnits = source["gpuUnits"];
	    }
	}
	export class Container {
	    name: string;
	    requests: ResourceValues;
	
	    static createFrom(source: any = {}) {
	        return new Container(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.requests = this.convertValues(source["requests"], ResourceValues);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NamespaceSummary {
	    name: string;
	    workloadCount: number;
	    podCount: number;
	    missingRequests: number;
	    requests: ResourceValues;
	
	    static createFrom(source: any = {}) {
	        return new NamespaceSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.workloadCount = source["workloadCount"];
	        this.podCount = source["podCount"];
	        this.missingRequests = source["missingRequests"];
	        this.requests = this.convertValues(source["requests"], ResourceValues);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Node {
	    uid: string;
	    name: string;
	    ready: boolean;
	    schedulable: boolean;
	    role: string;
	    zone: string;
	    region: string;
	    instanceType: string;
	    providerId: string;
	    capacity: ResourceValues;
	    allocatable: ResourceValues;
	    requests: ResourceValues;
	
	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.name = source["name"];
	        this.ready = source["ready"];
	        this.schedulable = source["schedulable"];
	        this.role = source["role"];
	        this.zone = source["zone"];
	        this.region = source["region"];
	        this.instanceType = source["instanceType"];
	        this.providerId = source["providerId"];
	        this.capacity = this.convertValues(source["capacity"], ResourceValues);
	        this.allocatable = this.convertValues(source["allocatable"], ResourceValues);
	        this.requests = this.convertValues(source["requests"], ResourceValues);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Resource {
	    uid: string;
	    kind: string;
	    name: string;
	    namespace: string;
	    category: string;
	    status: string;
	    attributes: Record<string, string>;
	    requests: ResourceValues;
	
	    static createFrom(source: any = {}) {
	        return new Resource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.category = source["category"];
	        this.status = source["status"];
	        this.attributes = source["attributes"];
	        this.requests = this.convertValues(source["requests"], ResourceValues);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Warning {
	    resource: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Warning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resource = source["resource"];
	        this.message = source["message"];
	    }
	}
	export class Workload {
	    uid: string;
	    kind: string;
	    name: string;
	    namespace: string;
	    desiredReplicas: number;
	    readyReplicas: number;
	    containers: Container[];
	    requests: ResourceValues;
	    missingRequests: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Workload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.desiredReplicas = source["desiredReplicas"];
	        this.readyReplicas = source["readyReplicas"];
	        this.containers = this.convertValues(source["containers"], Container);
	        this.requests = this.convertValues(source["requests"], ResourceValues);
	        this.missingRequests = source["missingRequests"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Summary {
	    nodeCount: number;
	    readyNodeCount: number;
	    workloadCount: number;
	    resourceCount: number;
	    namespaceCount: number;
	    missingRequestWorkloads: number;
	    requests: ResourceValues;
	    allocatable: ResourceValues;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeCount = source["nodeCount"];
	        this.readyNodeCount = source["readyNodeCount"];
	        this.workloadCount = source["workloadCount"];
	        this.resourceCount = source["resourceCount"];
	        this.namespaceCount = source["namespaceCount"];
	        this.missingRequestWorkloads = source["missingRequestWorkloads"];
	        this.requests = this.convertValues(source["requests"], ResourceValues);
	        this.allocatable = this.convertValues(source["allocatable"], ResourceValues);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Snapshot {
	    schemaVersion: number;
	    collectedAt: number;
	    cluster: ClusterInfo;
	    summary: Summary;
	    nodes: Node[];
	    workloads: Workload[];
	    resources: Resource[];
	    namespaces: NamespaceSummary[];
	    warnings: Warning[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.collectedAt = source["collectedAt"];
	        this.cluster = this.convertValues(source["cluster"], ClusterInfo);
	        this.summary = this.convertValues(source["summary"], Summary);
	        this.nodes = this.convertValues(source["nodes"], Node);
	        this.workloads = this.convertValues(source["workloads"], Workload);
	        this.resources = this.convertValues(source["resources"], Resource);
	        this.namespaces = this.convertValues(source["namespaces"], NamespaceSummary);
	        this.warnings = this.convertValues(source["warnings"], Warning);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

export namespace wails {
	
	export class ClusterConnectionRequest {
	    kubeconfigPath: string;
	    context: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterConnectionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.context = source["context"];
	    }
	}
	export class ClusterConnectionResult {
	    context: string;
	    server: string;
	    version: string;
	    connectedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ClusterConnectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.server = source["server"];
	        this.version = source["version"];
	        this.connectedAt = source["connectedAt"];
	    }
	}
	export class ClusterSnapshotRequest {
	    kubeconfigPath: string;
	    context: string;
	    namespace: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterSnapshotRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.context = source["context"];
	        this.namespace = source["namespace"];
	    }
	}
	export class KubeconfigContext {
	    name: string;
	    server: string;
	    cluster: string;
	
	    static createFrom(source: any = {}) {
	        return new KubeconfigContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.server = source["server"];
	        this.cluster = source["cluster"];
	    }
	}
	export class KubeconfigContextsRequest {
	    kubeconfigPath: string;
	
	    static createFrom(source: any = {}) {
	        return new KubeconfigContextsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	    }
	}
	export class ManifestDocument {
	    name: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestDocument(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.content = source["content"];
	    }
	}
	export class ManifestRequest {
	    documents: ManifestDocument[];
	    provider: string;
	    region: string;
	    instanceType: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.documents = this.convertValues(source["documents"], ManifestDocument);
	        this.provider = source["provider"];
	        this.region = source["region"];
	        this.instanceType = source["instanceType"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PricingResult {
	    provider: string;
	    region: string;
	    instanceType: string;
	
	    static createFrom(source: any = {}) {
	        return new PricingResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.region = source["region"];
	        this.instanceType = source["instanceType"];
	    }
	}
	export class ResourceResult {
	    name: string;
	    cpuCores: number;
	    memoryGB: number;
	    storageGB: number;
	    gpuUnits: number;
	    hourlyCost: number;
	
	    static createFrom(source: any = {}) {
	        return new ResourceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cpuCores = source["cpuCores"];
	        this.memoryGB = source["memoryGB"];
	        this.storageGB = source["storageGB"];
	        this.gpuUnits = source["gpuUnits"];
	        this.hourlyCost = source["hourlyCost"];
	    }
	}
	export class WorkloadResult {
	    name: string;
	    namespace: string;
	    replicas: number;
	    minReplicas?: number;
	    maxReplicas?: number;
	    resources: ResourceResult[];
	
	    static createFrom(source: any = {}) {
	        return new WorkloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.replicas = source["replicas"];
	        this.minReplicas = source["minReplicas"];
	        this.maxReplicas = source["maxReplicas"];
	        this.resources = this.convertValues(source["resources"], ResourceResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ManifestResult {
	    workload: WorkloadResult;
	    pricing: PricingResult;
	    hourlyTotal: number;
	    dailyTotal: number;
	    monthlyTotal: number;
	    currency: string;
	    minTotal?: number;
	    maxTotal?: number;
	
	    static createFrom(source: any = {}) {
	        return new ManifestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workload = this.convertValues(source["workload"], WorkloadResult);
	        this.pricing = this.convertValues(source["pricing"], PricingResult);
	        this.hourlyTotal = source["hourlyTotal"];
	        this.dailyTotal = source["dailyTotal"];
	        this.monthlyTotal = source["monthlyTotal"];
	        this.currency = source["currency"];
	        this.minTotal = source["minTotal"];
	        this.maxTotal = source["maxTotal"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class WorkloadYAMLRequest {
	    kubeconfigPath: string;
	    context: string;
	    kind: string;
	    namespace: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkloadYAMLRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.context = source["context"];
	        this.kind = source["kind"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	    }
	}

}

