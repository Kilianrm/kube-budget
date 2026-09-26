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
	    taints?: string[];
	
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
	        this.taints = source["taints"];
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
	export class Platform {
	    provider: string;
	    detected: string;
	    region?: string;
	    supported: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new Platform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.detected = source["detected"];
	        this.region = source["region"];
	        this.supported = source["supported"];
	        this.reason = source["reason"];
	    }
	}
	export class Pod {
	    name: string;
	    namespace: string;
	    nodeName?: string;
	    phase: string;
	    ownerKind: string;
	    ownerName: string;
	    requests: ResourceValues;
	    missingRequests: boolean;
	    missingResources?: string[];
	    usage?: ResourceValues;
	    claims?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Pod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.nodeName = source["nodeName"];
	        this.phase = source["phase"];
	        this.ownerKind = source["ownerKind"];
	        this.ownerName = source["ownerName"];
	        this.requests = this.convertValues(source["requests"], ResourceValues);
	        this.missingRequests = source["missingRequests"];
	        this.missingResources = source["missingResources"];
	        this.usage = this.convertValues(source["usage"], ResourceValues);
	        this.claims = source["claims"];
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
	    missingResources?: string[];
	
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
	        this.missingResources = source["missingResources"];
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
	    pods: Pod[];
	    resources: Resource[];
	    namespaces: NamespaceSummary[];
	    warnings: Warning[];
	    provider?: ProviderMetadata;
	    platform: Platform;
	
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
	        this.pods = this.convertValues(source["pods"], Pod);
	        this.resources = this.convertValues(source["resources"], Resource);
	        this.namespaces = this.convertValues(source["namespaces"], NamespaceSummary);
	        this.warnings = this.convertValues(source["warnings"], Warning);
	        this.provider = this.convertValues(source["provider"], ProviderMetadata);
	        this.platform = this.convertValues(source["platform"], Platform);
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
	export class AllocationRow {
	    key: string;
	    name: string;
	    namespace?: string;
	    subjectKind?: string;
	    kind: string;
	    components?: Record<string, Projection>;
	    cost: Projection;
	    confidence: string;
	    items: number;
	    unpriced: number;
	    efficiency?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new AllocationRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.subjectKind = source["subjectKind"];
	        this.kind = source["kind"];
	        this.components = this.convertValues(source["components"], Projection, true);
	        this.cost = this.convertValues(source["cost"], Projection);
	        this.confidence = source["confidence"];
	        this.items = source["items"];
	        this.unpriced = source["unpriced"];
	        this.efficiency = source["efficiency"];
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
	export class UsageSummary {
	    used: Record<string, Projection>;
	    requested: Record<string, Projection>;
	    efficiency: Record<string, number>;
	    workloads: number;
	    withoutUsage: number;
	
	    static createFrom(source: any = {}) {
	        return new UsageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.used = this.convertValues(source["used"], Projection, true);
	        this.requested = this.convertValues(source["requested"], Projection, true);
	        this.efficiency = source["efficiency"];
	        this.workloads = source["workloads"];
	        this.withoutUsage = source["withoutUsage"];
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
	    used?: Usage;
	    usedComponents?: Record<string, number>;
	
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
	        this.used = this.convertValues(source["used"], Usage);
	        this.usedComponents = source["usedComponents"];
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
	    idleByComponent: Record<string, Projection>;
	    shared: Projection;
	    byDimension: Record<string, any>;
	    allocation: Record<string, Array<AllocationRow>>;
	    usage?: UsageSummary;
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
	        this.idleByComponent = this.convertValues(source["idleByComponent"], Projection, true);
	        this.shared = this.convertValues(source["shared"], Projection);
	        this.byDimension = source["byDimension"];
	        this.allocation = this.convertValues(source["allocation"], Array<AllocationRow>, true);
	        this.usage = this.convertValues(source["usage"], UsageSummary);
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
	
	export class BudgetStatus {
	    monthlyUSD: number;
	    state: string;
	    projectedRatio: number;
	    remainingUSD: number;
	    // Go type: time
	    exhaustedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new BudgetStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.monthlyUSD = source["monthlyUSD"];
	        this.state = source["state"];
	        this.projectedRatio = source["projectedRatio"];
	        this.remainingUSD = source["remainingUSD"];
	        this.exhaustedAt = this.convertValues(source["exhaustedAt"], null);
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
	export class Driver {
	    dimension: string;
	    key: string;
	    current: costmodel.Projection;
	    previous: costmodel.Projection;
	    delta: costmodel.Projection;
	
	    static createFrom(source: any = {}) {
	        return new Driver(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.key = source["key"];
	        this.current = this.convertValues(source["current"], costmodel.Projection);
	        this.previous = this.convertValues(source["previous"], costmodel.Projection);
	        this.delta = this.convertValues(source["delta"], costmodel.Projection);
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
	export class ForecastDay {
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	    recordedUSD: number;
	    estimatedUSD: number;
	    projectedUSD: number;
	    coveredHours: number;
	
	    static createFrom(source: any = {}) {
	        return new ForecastDay(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
	        this.recordedUSD = source["recordedUSD"];
	        this.estimatedUSD = source["estimatedUSD"];
	        this.projectedUSD = source["projectedUSD"];
	        this.coveredHours = source["coveredHours"];
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
	export class Forecast {
	    // Go type: time
	    monthStart: any;
	    // Go type: time
	    monthEnd: any;
	    // Go type: time
	    at: any;
	    recordedUSD: number;
	    estimatedUSD: number;
	    projectedUSD: number;
	    totalUSD: number;
	    runRateHourly: number;
	    elapsedHours: number;
	    coveredHours: number;
	    remainingHours: number;
	    days: ForecastDay[];
	
	    static createFrom(source: any = {}) {
	        return new Forecast(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.monthStart = this.convertValues(source["monthStart"], null);
	        this.monthEnd = this.convertValues(source["monthEnd"], null);
	        this.at = this.convertValues(source["at"], null);
	        this.recordedUSD = source["recordedUSD"];
	        this.estimatedUSD = source["estimatedUSD"];
	        this.projectedUSD = source["projectedUSD"];
	        this.totalUSD = source["totalUSD"];
	        this.runRateHourly = source["runRateHourly"];
	        this.elapsedHours = source["elapsedHours"];
	        this.coveredHours = source["coveredHours"];
	        this.remainingHours = source["remainingHours"];
	        this.days = this.convertValues(source["days"], ForecastDay);
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
	
	export class Point {
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	    provisionedUSD: number;
	    requestedUSD: number;
	    idleUSD: number;
	    sharedUSD: number;
	    coveredHours: number;
	    bucketHours: number;
	    byNamespace?: Record<string, number>;
	    byNodeGroup?: Record<string, number>;
	
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
	        this.sharedUSD = source["sharedUSD"];
	        this.coveredHours = source["coveredHours"];
	        this.bucketHours = source["bucketHours"];
	        this.byNamespace = source["byNamespace"];
	        this.byNodeGroup = source["byNodeGroup"];
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
	    totalSharedUSD: number;
	    totalByNamespace?: Record<string, number>;
	    totalByNodeGroup?: Record<string, number>;
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
	        this.totalSharedUSD = source["totalSharedUSD"];
	        this.totalByNamespace = source["totalByNamespace"];
	        this.totalByNodeGroup = source["totalByNodeGroup"];
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
	export class Summary {
	    provisionedUSD: number;
	    requestedUSD: number;
	    idleUSD: number;
	    sharedUSD: number;
	    coveredHours: number;
	    windowHours: number;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provisionedUSD = source["provisionedUSD"];
	        this.requestedUSD = source["requestedUSD"];
	        this.idleUSD = source["idleUSD"];
	        this.sharedUSD = source["sharedUSD"];
	        this.coveredHours = source["coveredHours"];
	        this.windowHours = source["windowHours"];
	    }
	}

}

export namespace optimize {
	
	export class Item {
	    subject: costmodel.Subject;
	    change: string;
	    savings: costmodel.Projection;
	    command?: string;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subject = this.convertValues(source["subject"], costmodel.Subject);
	        this.change = source["change"];
	        this.savings = this.convertValues(source["savings"], costmodel.Projection);
	        this.command = source["command"];
	        this.note = source["note"];
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
	export class Recommendation {
	    id: string;
	    category: string;
	    title: string;
	    effort: string;
	    risk: string;
	    confidence: string;
	    savings: costmodel.Projection;
	    freedRequests: costmodel.Projection;
	    rationale: string;
	    action: string;
	    dismissed: boolean;
	    items?: Item[];
	
	    static createFrom(source: any = {}) {
	        return new Recommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.title = source["title"];
	        this.effort = source["effort"];
	        this.risk = source["risk"];
	        this.confidence = source["confidence"];
	        this.savings = this.convertValues(source["savings"], costmodel.Projection);
	        this.freedRequests = this.convertValues(source["freedRequests"], costmodel.Projection);
	        this.rationale = source["rationale"];
	        this.action = source["action"];
	        this.dismissed = source["dismissed"];
	        this.items = this.convertValues(source["items"], Item);
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
	export class Step {
	    id: string;
	    title: string;
	    savings: costmodel.Projection;
	
	    static createFrom(source: any = {}) {
	        return new Step(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.savings = this.convertValues(source["savings"], costmodel.Projection);
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
	export class Plan {
	    current: costmodel.Projection;
	    optimized: costmodel.Projection;
	    savings: costmodel.Projection;
	    steps: Step[];
	    dataIssues: Recommendation[];
	    recommendations: Recommendation[];
	    assumptions: costmodel.Assumption[];
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = this.convertValues(source["current"], costmodel.Projection);
	        this.optimized = this.convertValues(source["optimized"], costmodel.Projection);
	        this.savings = this.convertValues(source["savings"], costmodel.Projection);
	        this.steps = this.convertValues(source["steps"], Step);
	        this.dataIssues = this.convertValues(source["dataIssues"], Recommendation);
	        this.recommendations = this.convertValues(source["recommendations"], Recommendation);
	        this.assumptions = this.convertValues(source["assumptions"], costmodel.Assumption);
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
	
	export class AppliedRecommendation {
	    id: string;
	    title: string;
	    // Go type: time
	    at: any;
	    pending: boolean;
	    // Go type: time
	    confirmedAt?: any;
	    expected: costmodel.Projection;
	    realized: costmodel.Projection;
	
	    static createFrom(source: any = {}) {
	        return new AppliedRecommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.at = this.convertValues(source["at"], null);
	        this.pending = source["pending"];
	        this.confirmedAt = this.convertValues(source["confirmedAt"], null);
	        this.expected = this.convertValues(source["expected"], costmodel.Projection);
	        this.realized = this.convertValues(source["realized"], costmodel.Projection);
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
	export class CostBudgetRequest {
	    clusterId: string;
	    monthlyUSD: number;
	
	    static createFrom(source: any = {}) {
	        return new CostBudgetRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterId = source["clusterId"];
	        this.monthlyUSD = source["monthlyUSD"];
	    }
	}
	export class CostForecastRequest {
	    clusterId: string;
	    runRateHourly: number;
	
	    static createFrom(source: any = {}) {
	        return new CostForecastRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterId = source["clusterId"];
	        this.runRateHourly = source["runRateHourly"];
	    }
	}
	export class CostForecastResult {
	    forecast: costseries.Forecast;
	    budget?: costseries.BudgetStatus;
	
	    static createFrom(source: any = {}) {
	        return new CostForecastResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.forecast = this.convertValues(source["forecast"], costseries.Forecast);
	        this.budget = this.convertValues(source["budget"], costseries.BudgetStatus);
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
	export class RecommendationEvent {
	    // Go type: time
	    at: any;
	    id: string;
	    title: string;
	    action: string;
	    savings: costmodel.Projection;
	
	    static createFrom(source: any = {}) {
	        return new RecommendationEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.action = source["action"];
	        this.savings = this.convertValues(source["savings"], costmodel.Projection);
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
	export class OptimizationResult {
	    plan: optimize.Plan;
	    applied: AppliedRecommendation[];
	    history: RecommendationEvent[];
	
	    static createFrom(source: any = {}) {
	        return new OptimizationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plan = this.convertValues(source["plan"], optimize.Plan);
	        this.applied = this.convertValues(source["applied"], AppliedRecommendation);
	        this.history = this.convertValues(source["history"], RecommendationEvent);
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
	export class CostReportResult {
	    report: costmodel.CostReport;
	    optimization: OptimizationResult;
	    clusterId: string;
	
	    static createFrom(source: any = {}) {
	        return new CostReportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report = this.convertValues(source["report"], costmodel.CostReport);
	        this.optimization = this.convertValues(source["optimization"], OptimizationResult);
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
	export class CostTrendResult {
	    series: costseries.Series;
	    previous: costseries.Summary;
	    drivers: costseries.Driver[];
	
	    static createFrom(source: any = {}) {
	        return new CostTrendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.series = this.convertValues(source["series"], costseries.Series);
	        this.previous = this.convertValues(source["previous"], costseries.Summary);
	        this.drivers = this.convertValues(source["drivers"], costseries.Driver);
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
	
	
	
	export class RecommendationRequest {
	    clusterId: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new RecommendationRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterId = source["clusterId"];
	        this.id = source["id"];
	    }
	}
	
	export class SimulationRequest {
	    clusterId: string;
	    document: ManifestDocument;
	
	    static createFrom(source: any = {}) {
	        return new SimulationRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterId = source["clusterId"];
	        this.document = this.convertValues(source["document"], ManifestDocument);
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
	export class SimulationResult {
	    estimate: ManifestResult;
	    impact: whatif.Result;
	    peak?: whatif.Result;
	    forecastBefore?: CostForecastResult;
	    forecastAfter?: CostForecastResult;
	    // Go type: time
	    reportAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SimulationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.estimate = this.convertValues(source["estimate"], ManifestResult);
	        this.impact = this.convertValues(source["impact"], whatif.Result);
	        this.peak = this.convertValues(source["peak"], whatif.Result);
	        this.forecastBefore = this.convertValues(source["forecastBefore"], CostForecastResult);
	        this.forecastAfter = this.convertValues(source["forecastAfter"], CostForecastResult);
	        this.reportAt = this.convertValues(source["reportAt"], null);
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

export namespace whatif {
	
	export class Placement {
	    node: string;
	    replicas: number;
	    new: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Placement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = source["node"];
	        this.replicas = source["replicas"];
	        this.new = source["new"];
	    }
	}
	export class PlanChange {
	    id: string;
	    title: string;
	    before: costmodel.Projection;
	    after: costmodel.Projection;
	
	    static createFrom(source: any = {}) {
	        return new PlanChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.before = this.convertValues(source["before"], costmodel.Projection);
	        this.after = this.convertValues(source["after"], costmodel.Projection);
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
	export class Workload {
	    name: string;
	    namespace: string;
	    kind: string;
	    replicas: number;
	    perReplica: costmodel.Usage;
	    containers: number;
	
	    static createFrom(source: any = {}) {
	        return new Workload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.replicas = source["replicas"];
	        this.perReplica = this.convertValues(source["perReplica"], costmodel.Usage);
	        this.containers = source["containers"];
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
	export class Result {
	    workload: Workload;
	    fits: boolean;
	    placements: Placement[];
	    newNodes: number;
	    newNodeType?: string;
	    unschedulable: number;
	    requested: costmodel.Projection;
	    billedBefore: costmodel.Projection;
	    billedAfter: costmodel.Projection;
	    billedDelta: costmodel.Projection;
	    idleBefore: costmodel.Projection;
	    idleAfter: costmodel.Projection;
	    planSavingsBefore: costmodel.Projection;
	    planSavingsAfter: costmodel.Projection;
	    planChanges: PlanChange[];
	    warnings: string[];
	    assumptions: costmodel.Assumption[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workload = this.convertValues(source["workload"], Workload);
	        this.fits = source["fits"];
	        this.placements = this.convertValues(source["placements"], Placement);
	        this.newNodes = source["newNodes"];
	        this.newNodeType = source["newNodeType"];
	        this.unschedulable = source["unschedulable"];
	        this.requested = this.convertValues(source["requested"], costmodel.Projection);
	        this.billedBefore = this.convertValues(source["billedBefore"], costmodel.Projection);
	        this.billedAfter = this.convertValues(source["billedAfter"], costmodel.Projection);
	        this.billedDelta = this.convertValues(source["billedDelta"], costmodel.Projection);
	        this.idleBefore = this.convertValues(source["idleBefore"], costmodel.Projection);
	        this.idleAfter = this.convertValues(source["idleAfter"], costmodel.Projection);
	        this.planSavingsBefore = this.convertValues(source["planSavingsBefore"], costmodel.Projection);
	        this.planSavingsAfter = this.convertValues(source["planSavingsAfter"], costmodel.Projection);
	        this.planChanges = this.convertValues(source["planChanges"], PlanChange);
	        this.warnings = source["warnings"];
	        this.assumptions = this.convertValues(source["assumptions"], costmodel.Assumption);
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

