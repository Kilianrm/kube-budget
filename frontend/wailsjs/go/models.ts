export namespace cluster {
	
	export class Addon {
	    name: string;
	    version: string;
	    status: string;
	    health: string;
	    serviceAccount: string;
	
	    static createFrom(source: any = {}) {
	        return new Addon(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.status = source["status"];
	        this.health = source["health"];
	        this.serviceAccount = source["serviceAccount"];
	    }
	}
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
	export class NodeGroup {
	    name: string;
	    status: string;
	    instanceTypes: string[];
	    capacityType: string;
	    desiredSize: number;
	    minSize: number;
	    maxSize: number;
	    amiType: string;
	    nodeRole: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.instanceTypes = source["instanceTypes"];
	        this.capacityType = source["capacityType"];
	        this.desiredSize = source["desiredSize"];
	        this.minSize = source["minSize"];
	        this.maxSize = source["maxSize"];
	        this.amiType = source["amiType"];
	        this.nodeRole = source["nodeRole"];
	    }
	}
	export class ProviderMetadata {
	    provider: string;
	    clusterName: string;
	    clusterArn: string;
	    accountId: string;
	    region: string;
	    status: string;
	    kubernetesVersion: string;
	    platformVersion: string;
	    createdAt: number;
	    endpointAccess: string;
	    vpcId: string;
	    subnetIds: string[];
	    securityGroupIds: string[];
	    authenticationMode: string;
	    nodeGroups: NodeGroup[];
	    addons: Addon[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.clusterName = source["clusterName"];
	        this.clusterArn = source["clusterArn"];
	        this.accountId = source["accountId"];
	        this.region = source["region"];
	        this.status = source["status"];
	        this.kubernetesVersion = source["kubernetesVersion"];
	        this.platformVersion = source["platformVersion"];
	        this.createdAt = source["createdAt"];
	        this.endpointAccess = source["endpointAccess"];
	        this.vpcId = source["vpcId"];
	        this.subnetIds = source["subnetIds"];
	        this.securityGroupIds = source["securityGroupIds"];
	        this.authenticationMode = source["authenticationMode"];
	        this.nodeGroups = this.convertValues(source["nodeGroups"], NodeGroup);
	        this.addons = this.convertValues(source["addons"], Addon);
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
	    provider?: ProviderMetadata;
	
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
	        this.provider = this.convertValues(source["provider"], ProviderMetadata);
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

export namespace costmodel {
	
	export class Assumption {
	    key: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Assumption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.detail = source["detail"];
	    }
	}
	export class Warning {
	    code: string;
	    message: string;
	    subject?: string;
	
	    static createFrom(source: any = {}) {
	        return new Warning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.subject = source["subject"];
	    }
	}
	export class Projection {
	    hourly: number;
	    daily: number;
	    monthly: number;
	    yearly: number;
	
	    static createFrom(source: any = {}) {
	        return new Projection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hourly = source["hourly"];
	        this.daily = source["daily"];
	        this.monthly = source["monthly"];
	        this.yearly = source["yearly"];
	    }
	}
	export class Usage {
	    cpuCores: number;
	    memoryGB: number;
	    storageGB: number;
	    gpuUnits: number;
	
	    static createFrom(source: any = {}) {
	        return new Usage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuCores = source["cpuCores"];
	        this.memoryGB = source["memoryGB"];
	        this.storageGB = source["storageGB"];
	        this.gpuUnits = source["gpuUnits"];
	    }
	}
	export class Subject {
	    kind: string;
	    id: string;
	    name: string;
	    namespace?: string;
	    parentId?: string;
	    labels?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Subject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.parentId = source["parentId"];
	        this.labels = source["labels"];
	    }
	}
	export class LineItem {
	    subject: Subject;
	    basis: string;
	    usage: Usage;
	    components?: Record<string, number>;
	    hourlyUSD: number;
	    cost: Projection;
	    confidence: string;
	    assumptions?: Assumption[];
	    detail?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new LineItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subject = this.convertValues(source["subject"], Subject);
	        this.basis = source["basis"];
	        this.usage = this.convertValues(source["usage"], Usage);
	        this.components = source["components"];
	        this.hourlyUSD = source["hourlyUSD"];
	        this.cost = this.convertValues(source["cost"], Projection);
	        this.confidence = source["confidence"];
	        this.assumptions = this.convertValues(source["assumptions"], Assumption);
	        this.detail = source["detail"];
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
	export class Scope {
	    clusterName?: string;
	    provider?: string;
	    region?: string;
	    namespace?: string;
	
	    static createFrom(source: any = {}) {
	        return new Scope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterName = source["clusterName"];
	        this.provider = source["provider"];
	        this.region = source["region"];
	        this.namespace = source["namespace"];
	    }
	}
	export class CostReport {
	    // Go type: time
	    generatedAt: any;
	    scope: Scope;
	    currency: string;
	    items: LineItem[];
	    totals: Record<string, Projection>;
	    idle: Projection;
	    byDimension: Record<string, any>;
	    assumptions?: Assumption[];
	    warnings?: Warning[];
	
	    static createFrom(source: any = {}) {
	        return new CostReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	        this.scope = this.convertValues(source["scope"], Scope);
	        this.currency = source["currency"];
	        this.items = this.convertValues(source["items"], LineItem);
	        this.totals = this.convertValues(source["totals"], Projection, true);
	        this.idle = this.convertValues(source["idle"], Projection);
	        this.byDimension = source["byDimension"];
	        this.assumptions = this.convertValues(source["assumptions"], Assumption);
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

export namespace costseries {
	
	export class Point {
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	    provisionedUSD: number;
	    requestedUSD: number;
	    idleUSD: number;
	    coveredHours: number;
	    bucketHours: number;
	
	    static createFrom(source: any = {}) {
	        return new Point(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
	        this.provisionedUSD = source["provisionedUSD"];
	        this.requestedUSD = source["requestedUSD"];
	        this.idleUSD = source["idleUSD"];
	        this.coveredHours = source["coveredHours"];
	        this.bucketHours = source["bucketHours"];
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
	export class Series {
	    bucket: string;
	    currency: string;
	    // Go type: time
	    from: any;
	    // Go type: time
	    to: any;
	    points: Point[];
	    totalProvisionedUSD: number;
	    totalRequestedUSD: number;
	    totalIdleUSD: number;
	    coveredHours: number;
	    windowHours: number;
	
	    static createFrom(source: any = {}) {
	        return new Series(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bucket = source["bucket"];
	        this.currency = source["currency"];
	        this.from = this.convertValues(source["from"], null);
	        this.to = this.convertValues(source["to"], null);
	        this.points = this.convertValues(source["points"], Point);
	        this.totalProvisionedUSD = source["totalProvisionedUSD"];
	        this.totalRequestedUSD = source["totalRequestedUSD"];
	        this.totalIdleUSD = source["totalIdleUSD"];
	        this.coveredHours = source["coveredHours"];
	        this.windowHours = source["windowHours"];
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

export namespace optimize {
	
	export class Recommendation {
	    id: string;
	    rule: string;
	    title: string;
	    severity: string;
	    count: number;
	    subjects?: costmodel.Subject[];
	    current: costmodel.Projection;
	    proposed: costmodel.Projection;
	    savings: costmodel.Projection;
	    confidence: string;
	    rationale: string;
	    action: string;
	    detail?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Recommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rule = source["rule"];
	        this.title = source["title"];
	        this.severity = source["severity"];
	        this.count = source["count"];
	        this.subjects = this.convertValues(source["subjects"], costmodel.Subject);
	        this.current = this.convertValues(source["current"], costmodel.Projection);
	        this.proposed = this.convertValues(source["proposed"], costmodel.Projection);
	        this.savings = this.convertValues(source["savings"], costmodel.Projection);
	        this.confidence = source["confidence"];
	        this.rationale = source["rationale"];
	        this.action = source["action"];
	        this.detail = source["detail"];
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
	    provider: string;
	    clusterName: string;
	    region: string;
	    profile: string;
	    roleArn: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterSnapshotRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.context = source["context"];
	        this.namespace = source["namespace"];
	        this.provider = source["provider"];
	        this.clusterName = source["clusterName"];
	        this.region = source["region"];
	        this.profile = source["profile"];
	        this.roleArn = source["roleArn"];
	    }
	}
	export class CostReportRequest {
	    kubeconfigPath: string;
	    context: string;
	    namespace: string;
	    provider: string;
	    clusterName: string;
	    region: string;
	    profile: string;
	    roleArn: string;
	    pricingProvider: string;
	    pricingRegion: string;
	    instanceType: string;
	
	    static createFrom(source: any = {}) {
	        return new CostReportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.context = source["context"];
	        this.namespace = source["namespace"];
	        this.provider = source["provider"];
	        this.clusterName = source["clusterName"];
	        this.region = source["region"];
	        this.profile = source["profile"];
	        this.roleArn = source["roleArn"];
	        this.pricingProvider = source["pricingProvider"];
	        this.pricingRegion = source["pricingRegion"];
	        this.instanceType = source["instanceType"];
	    }
	}
	export class CostReportResult {
	    report: costmodel.CostReport;
	    recommendations: optimize.Recommendation[];
	    clusterId: string;
	
	    static createFrom(source: any = {}) {
	        return new CostReportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report = this.convertValues(source["report"], costmodel.CostReport);
	        this.recommendations = this.convertValues(source["recommendations"], optimize.Recommendation);
	        this.clusterId = source["clusterId"];
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
	export class CostTrendRequest {
	    clusterId: string;
	    days: number;
	    bucket: string;
	
	    static createFrom(source: any = {}) {
	        return new CostTrendRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterId = source["clusterId"];
	        this.days = source["days"];
	        this.bucket = source["bucket"];
	    }
	}
	export class EKSClustersRequest {
	    region: string;
	    profile: string;
	
	    static createFrom(source: any = {}) {
	        return new EKSClustersRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.region = source["region"];
	        this.profile = source["profile"];
	    }
	}
	export class EKSConnectionRequest {
	    kubeconfigPath: string;
	    clusterName: string;
	    region: string;
	    profile: string;
	    roleArn: string;
	
	    static createFrom(source: any = {}) {
	        return new EKSConnectionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.clusterName = source["clusterName"];
	        this.region = source["region"];
	        this.profile = source["profile"];
	        this.roleArn = source["roleArn"];
	    }
	}
	export class EKSConnectionResult {
	    kubeconfigPath: string;
	    context: string;
	
	    static createFrom(source: any = {}) {
	        return new EKSConnectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.context = source["context"];
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
	    cost: costmodel.Projection;
	    hourlyTotal: number;
	    dailyTotal: number;
	    monthlyTotal: number;
	    currency: string;
	    minTotal?: number;
	    maxTotal?: number;
	    minCost?: costmodel.Projection;
	    maxCost?: costmodel.Projection;
	
	    static createFrom(source: any = {}) {
	        return new ManifestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workload = this.convertValues(source["workload"], WorkloadResult);
	        this.pricing = this.convertValues(source["pricing"], PricingResult);
	        this.cost = this.convertValues(source["cost"], costmodel.Projection);
	        this.hourlyTotal = source["hourlyTotal"];
	        this.dailyTotal = source["dailyTotal"];
	        this.monthlyTotal = source["monthlyTotal"];
	        this.currency = source["currency"];
	        this.minTotal = source["minTotal"];
	        this.maxTotal = source["maxTotal"];
	        this.minCost = this.convertValues(source["minCost"], costmodel.Projection);
	        this.maxCost = this.convertValues(source["maxCost"], costmodel.Projection);
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

