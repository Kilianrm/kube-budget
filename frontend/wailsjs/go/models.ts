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
	
	

}

