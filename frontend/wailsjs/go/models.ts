export namespace app {
	
	export class AgentCall {
	    at: number;
	    client: string;
	    tool: string;
	    ok: boolean;
	    ms: number;
	
	    static createFrom(source: any = {}) {
	        return new AgentCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = source["at"];
	        this.client = source["client"];
	        this.tool = source["tool"];
	        this.ok = source["ok"];
	        this.ms = source["ms"];
	    }
	}
	export class AgentClient {
	    name: string;
	    since: number;
	
	    static createFrom(source: any = {}) {
	        return new AgentClient(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.since = source["since"];
	    }
	}
	export class AgentSetup {
	    id: string;
	    title: string;
	    text: string;
	    note: string;
	    file: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AgentSetup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.text = source["text"];
	        this.note = source["note"];
	        this.file = source["file"];
	    }
	}
	export class AgentsInfo {
	    status: string;
	    hint: string;
	    error: string;
	    level: number;
	    portable: boolean;
	    setups: AgentSetup[];
	    clients: AgentClient[];
	    calls: AgentCall[];
	    today: number;
	
	    static createFrom(source: any = {}) {
	        return new AgentsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.hint = source["hint"];
	        this.error = source["error"];
	        this.level = source["level"];
	        this.portable = source["portable"];
	        this.setups = this.convertValues(source["setups"], AgentSetup);
	        this.clients = this.convertValues(source["clients"], AgentClient);
	        this.calls = this.convertValues(source["calls"], AgentCall);
	        this.today = source["today"];
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
	export class AgentsSaved {
	    canceled: boolean;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentsSaved(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canceled = source["canceled"];
	        this.text = source["text"];
	    }
	}
	export class AssocView {
	    can: boolean;
	    on: boolean;
	    ask: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AssocView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.can = source["can"];
	        this.on = source["on"];
	        this.ask = source["ask"];
	    }
	}
	export class BackupInfo {
	    name: string;
	    time: number;
	    size: number;
	    spreads: number;
	    photos: number;
	
	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.time = source["time"];
	        this.size = source["size"];
	        this.spreads = source["spreads"];
	        this.photos = source["photos"];
	    }
	}
	export class BgResult {
	    view: model.View;
	    spreads: number;
	
	    static createFrom(source: any = {}) {
	        return new BgResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.spreads = source["spreads"];
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
	export class ClipResult {
	    view: model.View;
	    clip: model.ClipInfo;
	
	    static createFrom(source: any = {}) {
	        return new ClipResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.clip = this.convertValues(source["clip"], model.ClipInfo);
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
	export class CollectOptions {
	    all: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CollectOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.all = source["all"];
	    }
	}
	export class CoverResult {
	    view: model.View;
	    styled: boolean;
	    look: string;
	
	    static createFrom(source: any = {}) {
	        return new CoverResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.styled = source["styled"];
	        this.look = source["look"];
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
	export class DecorGet {
	    id: string;
	    pack: string;
	    src: string;
	    title: string;
	    url: string;
	    w: number;
	    h: number;
	    bytes: number;
	    with: string[];
	    bg: number;
	    sticker: number;
	    spread: number;
	    frame: number;
	
	    static createFrom(source: any = {}) {
	        return new DecorGet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pack = source["pack"];
	        this.src = source["src"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.bytes = source["bytes"];
	        this.with = source["with"];
	        this.bg = source["bg"];
	        this.sticker = source["sticker"];
	        this.spread = source["spread"];
	        this.frame = source["frame"];
	    }
	}
	export class DecorItem {
	    id: string;
	    theme: string;
	    name: string;
	    url: string;
	    own: boolean;
	    aspect: number;
	    w: number;
	    h: number;
	    color?: string;
	    dark?: boolean;
	    pages?: number;
	    places?: number;
	    pack?: string;
	    steps?: number;
	    from?: number;
	    vig?: boolean;
	    cover?: boolean;
	    packId?: string;
	    peek?: string;
	    form?: string;
	    style?: string;
	
	    static createFrom(source: any = {}) {
	        return new DecorItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.theme = source["theme"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.own = source["own"];
	        this.aspect = source["aspect"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.color = source["color"];
	        this.dark = source["dark"];
	        this.pages = source["pages"];
	        this.places = source["places"];
	        this.pack = source["pack"];
	        this.steps = source["steps"];
	        this.from = source["from"];
	        this.vig = source["vig"];
	        this.cover = source["cover"];
	        this.packId = source["packId"];
	        this.peek = source["peek"];
	        this.form = source["form"];
	        this.style = source["style"];
	    }
	}
	export class DecorTheme {
	    id: string;
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new DecorTheme(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}
	export class DecorCatalog {
	    kind: string;
	    themes: DecorTheme[];
	    items: DecorItem[];
	    recent: string[];
	    new: string[];
	    readOnly: boolean;
	    demoOnly: boolean;
	    get: DecorGet[];
	    invite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DecorCatalog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.themes = this.convertValues(source["themes"], DecorTheme);
	        this.items = this.convertValues(source["items"], DecorItem);
	        this.recent = source["recent"];
	        this.new = source["new"];
	        this.readOnly = source["readOnly"];
	        this.demoOnly = source["demoOnly"];
	        this.get = this.convertValues(source["get"], DecorGet);
	        this.invite = source["invite"];
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
	
	export class DecorSvg {
	    token: string;
	    name: string;
	    data: string;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new DecorSvg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.name = source["name"];
	        this.data = source["data"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	export class DecorSkip {
	    name: string;
	    why: string;
	
	    static createFrom(source: any = {}) {
	        return new DecorSkip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.why = source["why"];
	    }
	}
	export class DecorImport {
	    added: DecorItem[];
	    skipped: DecorSkip[];
	    svg: DecorSvg[];
	
	    static createFrom(source: any = {}) {
	        return new DecorImport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = this.convertValues(source["added"], DecorItem);
	        this.skipped = this.convertValues(source["skipped"], DecorSkip);
	        this.svg = this.convertValues(source["svg"], DecorSvg);
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
	
	
	
	
	export class DriveHint {
	    from: string;
	    to: string;
	    found: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new DriveHint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.found = source["found"];
	        this.total = source["total"];
	    }
	}
	export class DropResult {
	    view: model.View;
	    ask: boolean;
	    fit: number;
	    total: number;
	    fresh: string[];
	    grown: number;
	    out: boolean;
	    left: number;
	    small: boolean;
	    named: string[];
	
	    static createFrom(source: any = {}) {
	        return new DropResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.ask = source["ask"];
	        this.fit = source["fit"];
	        this.total = source["total"];
	        this.fresh = source["fresh"];
	        this.grown = source["grown"];
	        this.out = source["out"];
	        this.left = source["left"];
	        this.small = source["small"];
	        this.named = source["named"];
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
	export class ExportOptions {
	    range: string;
	    quality: number;
	    dpi: number;
	    pages: boolean;
	    cover: boolean;
	    coverOnly: boolean;
	    print: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.range = source["range"];
	        this.quality = source["quality"];
	        this.dpi = source["dpi"];
	        this.pages = source["pages"];
	        this.cover = source["cover"];
	        this.coverOnly = source["coverOnly"];
	        this.print = source["print"];
	    }
	}
	export class FacesResult {
	    view: model.View;
	    report: model.FaceReport;
	
	    static createFrom(source: any = {}) {
	        return new FacesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.report = this.convertValues(source["report"], model.FaceReport);
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
	export class FontAdded {
	    id: string;
	    name: string;
	    faces: number;
	    noCyr: boolean;
	    files: string[];
	
	    static createFrom(source: any = {}) {
	        return new FontAdded(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.faces = source["faces"];
	        this.noCyr = source["noCyr"];
	        this.files = source["files"];
	    }
	}
	export class FontImport {
	    added: FontAdded[];
	    faces: string[];
	    skipped: DecorSkip[];
	
	    static createFrom(source: any = {}) {
	        return new FontImport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = this.convertValues(source["added"], FontAdded);
	        this.faces = source["faces"];
	        this.skipped = this.convertValues(source["skipped"], DecorSkip);
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
	export class GridResult {
	    view: model.View;
	    left: number;
	    named: string[];
	
	    static createFrom(source: any = {}) {
	        return new GridResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.left = source["left"];
	        this.named = source["named"];
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
	export class GuideVideo {
	    id: string;
	    version: number;
	    seconds: number;
	    size: number;
	    ready: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GuideVideo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.version = source["version"];
	        this.seconds = source["seconds"];
	        this.size = source["size"];
	        this.ready = source["ready"];
	    }
	}
	export class LibraryNet {
	    allowed: boolean;
	    atStart: boolean;
	    checked: number;
	    skipped: number;
	    demoSkipped: boolean;
	    demoBack: boolean;
	    demoOffer: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LibraryNet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowed = source["allowed"];
	        this.atStart = source["atStart"];
	        this.checked = source["checked"];
	        this.skipped = source["skipped"];
	        this.demoSkipped = source["demoSkipped"];
	        this.demoBack = source["demoBack"];
	        this.demoOffer = source["demoOffer"];
	    }
	}
	export class LibraryOffer {
	    count: number;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryOffer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.bytes = source["bytes"];
	    }
	}
	export class LibraryUpdate {
	    added: number;
	    updated: number;
	    skipped: DecorSkip[];
	    bytes: number;
	    failed: DecorSkip[];
	    available: number;
	    fresh: number;
	    picked: boolean;
	    why: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.updated = source["updated"];
	        this.skipped = this.convertValues(source["skipped"], DecorSkip);
	        this.bytes = source["bytes"];
	        this.failed = this.convertValues(source["failed"], DecorSkip);
	        this.available = source["available"];
	        this.fresh = source["fresh"];
	        this.picked = source["picked"];
	        this.why = source["why"];
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
	export class LookCount {
	    n: number;
	    places: number;
	    fit: number;
	
	    static createFrom(source: any = {}) {
	        return new LookCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.n = source["n"];
	        this.places = source["places"];
	        this.fit = source["fit"];
	    }
	}
	export class LookStepInfo {
	    name: string;
	    role: string;
	    places: number;
	    pages: number;
	
	    static createFrom(source: any = {}) {
	        return new LookStepInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.role = source["role"];
	        this.places = source["places"];
	        this.pages = source["pages"];
	    }
	}
	export class LookInfo {
	    steps: LookStepInfo[];
	    cover: boolean;
	    min: number;
	    max: number;
	
	    static createFrom(source: any = {}) {
	        return new LookInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steps = this.convertValues(source["steps"], LookStepInfo);
	        this.cover = source["cover"];
	        this.min = source["min"];
	        this.max = source["max"];
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
	export class LookPlanSave {
	    ok: boolean;
	    steps: number;
	    max: number;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new LookPlanSave(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.steps = source["steps"];
	        this.max = source["max"];
	        this.reason = source["reason"];
	    }
	}
	export class LookResult {
	    view: model.View;
	    plan: model.LookPlan;
	
	    static createFrom(source: any = {}) {
	        return new LookResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.plan = this.convertValues(source["plan"], model.LookPlan);
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
	
	export class LooseResult {
	    view: model.View;
	    batch: string;
	    added: number;
	    inFolder: number;
	    folder: string;
	    dup: number;
	    over: number;
	    skipped: number;
	    bad: number;
	
	    static createFrom(source: any = {}) {
	        return new LooseResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.batch = source["batch"];
	        this.added = source["added"];
	        this.inFolder = source["inFolder"];
	        this.folder = source["folder"];
	        this.dup = source["dup"];
	        this.over = source["over"];
	        this.skipped = source["skipped"];
	        this.bad = source["bad"];
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
	export class NamesFile {
	    text: string;
	    file: string;
	    sheet: string;
	    sheets: number;
	    rows: number;
	    cancelled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NamesFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.file = source["file"];
	        this.sheet = source["sheet"];
	        this.sheets = source["sheets"];
	        this.rows = source["rows"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class NamesResult {
	    view: model.View;
	    extra: number;
	    missing: number;
	    named: number;
	    seats: number;
	
	    static createFrom(source: any = {}) {
	        return new NamesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.extra = source["extra"];
	        this.missing = source["missing"];
	        this.named = source["named"];
	        this.seats = source["seats"];
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
	export class NewerInfo {
	    name: string;
	    draft: boolean;
	    need: string;
	    mine: string;
	    have: string;
	
	    static createFrom(source: any = {}) {
	        return new NewerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.draft = source["draft"];
	        this.need = source["need"];
	        this.mine = source["mine"];
	        this.have = source["have"];
	    }
	}
	export class PackInfo {
	    id: string;
	    title: string;
	    about: string;
	    author: string;
	    license: string;
	    source: string;
	    origin: string;
	    version: number;
	    bg: number;
	    sticker: number;
	    spread: number;
	    frame: number;
	    print: number;
	    bytes: number;
	    removable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.about = source["about"];
	        this.author = source["author"];
	        this.license = source["license"];
	        this.source = source["source"];
	        this.origin = source["origin"];
	        this.version = source["version"];
	        this.bg = source["bg"];
	        this.sticker = source["sticker"];
	        this.spread = source["spread"];
	        this.frame = source["frame"];
	        this.print = source["print"];
	        this.bytes = source["bytes"];
	        this.removable = source["removable"];
	    }
	}
	export class PackAdd {
	    pack: PackInfo;
	    ask: string;
	    fromVersion: number;
	    token: string;
	    skipped: DecorSkip[];
	
	    static createFrom(source: any = {}) {
	        return new PackAdd(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pack = this.convertValues(source["pack"], PackInfo);
	        this.ask = source["ask"];
	        this.fromVersion = source["fromVersion"];
	        this.token = source["token"];
	        this.skipped = this.convertValues(source["skipped"], DecorSkip);
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
	export class PackAvail {
	    id: string;
	    title: string;
	    about: string;
	    license: string;
	    version: number;
	    bg: number;
	    sticker: number;
	    spread: number;
	    frame: number;
	    print: number;
	    bytes: number;
	    addonTitles: string[];
	    donor: string;
	    why: string;
	    removed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackAvail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.about = source["about"];
	        this.license = source["license"];
	        this.version = source["version"];
	        this.bg = source["bg"];
	        this.sticker = source["sticker"];
	        this.spread = source["spread"];
	        this.frame = source["frame"];
	        this.print = source["print"];
	        this.bytes = source["bytes"];
	        this.addonTitles = source["addonTitles"];
	        this.donor = source["donor"];
	        this.why = source["why"];
	        this.removed = source["removed"];
	    }
	}
	
	export class PackList {
	    packs: PackInfo[];
	    net: LibraryNet;
	    busy: boolean;
	    readOnly: boolean;
	    available: PackAvail[];
	    all: LibraryOffer;
	    availAt: number;
	
	    static createFrom(source: any = {}) {
	        return new PackList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packs = this.convertValues(source["packs"], PackInfo);
	        this.net = this.convertValues(source["net"], LibraryNet);
	        this.busy = source["busy"];
	        this.readOnly = source["readOnly"];
	        this.available = this.convertValues(source["available"], PackAvail);
	        this.all = this.convertValues(source["all"], LibraryOffer);
	        this.availAt = source["availAt"];
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
	export class PackRemove {
	    files: number;
	    kept: number;
	    bytes: number;
	    addons: string[];
	
	    static createFrom(source: any = {}) {
	        return new PackRemove(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.kept = source["kept"];
	        this.bytes = source["bytes"];
	        this.addons = source["addons"];
	    }
	}
	export class PagePxView {
	    pageW: number;
	    pageH: number;
	    err: string;
	
	    static createFrom(source: any = {}) {
	        return new PagePxView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pageW = source["pageW"];
	        this.pageH = source["pageH"];
	        this.err = source["err"];
	    }
	}
	export class PasteResult {
	    view: model.View;
	    refs: string[];
	    texts: string[];
	    stickers: string[];
	    repeat: string[];
	    froze: boolean;
	    clip: model.ClipInfo;
	
	    static createFrom(source: any = {}) {
	        return new PasteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.refs = source["refs"];
	        this.texts = source["texts"];
	        this.stickers = source["stickers"];
	        this.repeat = source["repeat"];
	        this.froze = source["froze"];
	        this.clip = this.convertValues(source["clip"], model.ClipInfo);
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
	export class PhotoAbout {
	    id: string;
	    file: string;
	    shot: string[];
	    note: string;
	    used: string;
	    uses: model.PhotoUse[];
	
	    static createFrom(source: any = {}) {
	        return new PhotoAbout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.file = source["file"];
	        this.shot = source["shot"];
	        this.note = source["note"];
	        this.used = source["used"];
	        this.uses = this.convertValues(source["uses"], model.PhotoUse);
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
	export class PrintCheckRow {
	    pageW: number;
	    pageH: number;
	    sideW: number;
	    sideH: number;
	    err: string;
	    field: string;
	    hit: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrintCheckRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pageW = source["pageW"];
	        this.pageH = source["pageH"];
	        this.sideW = source["sideW"];
	        this.sideH = source["sideH"];
	        this.err = source["err"];
	        this.field = source["field"];
	        this.hit = source["hit"];
	    }
	}
	export class PrintEditFormat {
	    name: string;
	    pxW: number;
	    pxH: number;
	    coverW: number;
	    coverH: number;
	
	    static createFrom(source: any = {}) {
	        return new PrintEditFormat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pxW = source["pxW"];
	        this.pxH = source["pxH"];
	        this.coverW = source["coverW"];
	        this.coverH = source["coverH"];
	    }
	}
	export class PrintEdit {
	    name: string;
	    base: string;
	    dpi: number;
	    layflat: boolean;
	    safe: number;
	    bleed: number;
	    formats: PrintEditFormat[];
	    spreads: number;
	    prefix: string;
	    quality: number;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintEdit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.base = source["base"];
	        this.dpi = source["dpi"];
	        this.layflat = source["layflat"];
	        this.safe = source["safe"];
	        this.bleed = source["bleed"];
	        this.formats = this.convertValues(source["formats"], PrintEditFormat);
	        this.spreads = source["spreads"];
	        this.prefix = source["prefix"];
	        this.quality = source["quality"];
	        this.url = source["url"];
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
	export class PrintEditCheck {
	    ok: boolean;
	    errs: Record<string, string>;
	    formats: PrintCheckRow[];
	    spine: number;
	    lines: string[];
	    names: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintEditCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.errs = source["errs"];
	        this.formats = this.convertValues(source["formats"], PrintCheckRow);
	        this.spine = source["spine"];
	        this.lines = source["lines"];
	        this.names = source["names"];
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
	
	export class PrintImport {
	    canceled: boolean;
	    info: printprof.Info;
	    ask: string;
	    question: string;
	    token: string;
	    copied: boolean;
	    lines: string[];
	
	    static createFrom(source: any = {}) {
	        return new PrintImport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canceled = source["canceled"];
	        this.info = this.convertValues(source["info"], printprof.Info);
	        this.ask = source["ask"];
	        this.question = source["question"];
	        this.token = source["token"];
	        this.copied = source["copied"];
	        this.lines = source["lines"];
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
	export class PrintSaved {
	    id: string;
	    view: model.View;
	
	    static createFrom(source: any = {}) {
	        return new PrintSaved(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.view = this.convertValues(source["view"], model.View);
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
	export class PrintShare {
	    canceled: boolean;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintShare(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canceled = source["canceled"];
	        this.name = source["name"];
	    }
	}
	export class QRInfo {
	    text: string;
	    side: number;
	    path: string;
	    minMM: number;
	
	    static createFrom(source: any = {}) {
	        return new QRInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.side = source["side"];
	        this.path = source["path"];
	        this.minMM = source["minMM"];
	    }
	}
	export class RelinkResult {
	    view: model.View;
	    found: number;
	    missing: number;
	    ambiguous: number;
	
	    static createFrom(source: any = {}) {
	        return new RelinkResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.found = source["found"];
	        this.missing = source["missing"];
	        this.ambiguous = source["ambiguous"];
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
	export class ShareAlbum {
	    block: string;
	    save: boolean;
	    title: string;
	    pages: number;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ShareAlbum(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.block = source["block"];
	        this.save = source["save"];
	        this.title = source["title"];
	        this.pages = source["pages"];
	        this.bytes = source["bytes"];
	    }
	}
	export class ShareChange {
	    days: number;
	    setPassword: boolean;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new ShareChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.days = source["days"];
	        this.setPassword = source["setPassword"];
	        this.password = source["password"];
	    }
	}
	export class ShareLink {
	    state: string;
	    url: string;
	    title: string;
	    password: string;
	    expiresAt: string;
	    until: string;
	    host: string;
	    published: string;
	
	    static createFrom(source: any = {}) {
	        return new ShareLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.password = source["password"];
	        this.expiresAt = source["expiresAt"];
	        this.until = source["until"];
	        this.host = source["host"];
	        this.published = source["published"];
	    }
	}
	export class ShareOptions {
	    title: string;
	    password: string;
	    days: number;
	
	    static createFrom(source: any = {}) {
	        return new ShareOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.password = source["password"];
	        this.days = source["days"];
	    }
	}
	export class ShareService {
	    host: string;
	    url: string;
	    token: boolean;
	    store: string;
	    days: number;
	
	    static createFrom(source: any = {}) {
	        return new ShareService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.url = source["url"];
	        this.token = source["token"];
	        this.store = source["store"];
	        this.days = source["days"];
	    }
	}
	export class ShareView {
	    service: ShareService;
	    album: ShareAlbum;
	    link?: ShareLink;
	    busy: boolean;
	    busyOther: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ShareView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.service = this.convertValues(source["service"], ShareService);
	        this.album = this.convertValues(source["album"], ShareAlbum);
	        this.link = this.convertValues(source["link"], ShareLink);
	        this.busy = source["busy"];
	        this.busyOther = source["busyOther"];
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
	export class SigState {
	    done: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new SigState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.done = source["done"];
	        this.total = source["total"];
	    }
	}
	export class SimilarResult {
	    ids: string[];
	    more: number;
	    why: string;
	
	    static createFrom(source: any = {}) {
	        return new SimilarResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ids = source["ids"];
	        this.more = source["more"];
	        this.why = source["why"];
	    }
	}
	export class State {
	    view: model.View;
	    photos: library.Photo[];
	    scan: library.ScanProgress;
	    notice?: string;
	    info?: string;
	    freshFonts?: string[];
	    missing: string[];
	    unread?: Record<string, string>;
	    version: string;
	    fonts: text.Family[];
	    prefs: prefs.Prefs;
	    recent: prefs.RecentProject[];
	    keymap: keymap.View;
	    cancelled?: boolean;
	    pending?: string;
	    newer?: NewerInfo;
	    assoc: AssocView;
	    fromFile?: boolean;
	    stand?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.photos = this.convertValues(source["photos"], library.Photo);
	        this.scan = this.convertValues(source["scan"], library.ScanProgress);
	        this.notice = source["notice"];
	        this.info = source["info"];
	        this.freshFonts = source["freshFonts"];
	        this.missing = source["missing"];
	        this.unread = source["unread"];
	        this.version = source["version"];
	        this.fonts = this.convertValues(source["fonts"], text.Family);
	        this.prefs = this.convertValues(source["prefs"], prefs.Prefs);
	        this.recent = this.convertValues(source["recent"], prefs.RecentProject);
	        this.keymap = this.convertValues(source["keymap"], keymap.View);
	        this.cancelled = source["cancelled"];
	        this.pending = source["pending"];
	        this.newer = this.convertValues(source["newer"], NewerInfo);
	        this.assoc = this.convertValues(source["assoc"], AssocView);
	        this.fromFile = source["fromFile"];
	        this.stand = source["stand"];
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
	export class StickerResult {
	    view: model.View;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new StickerResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.id = source["id"];
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
	export class TplLoad {
	    canceled: boolean;
	    added: number;
	    same: number;
	    over: number;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new TplLoad(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canceled = source["canceled"];
	        this.added = source["added"];
	        this.same = source["same"];
	        this.over = source["over"];
	        this.text = source["text"];
	    }
	}
	export class TplResult {
	    view: model.View;
	    left: number;
	    missing: number;
	
	    static createFrom(source: any = {}) {
	        return new TplResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.left = source["left"];
	        this.missing = source["missing"];
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
	export class TplSave {
	    canceled: boolean;
	    saved: number;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new TplSave(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canceled = source["canceled"];
	        this.saved = source["saved"];
	        this.text = source["text"];
	    }
	}
	export class UpdateInfo {
	    phase: string;
	    mode: string;
	    version: string;
	    notes: string[];
	    done: number;
	    total: number;
	    error: string;
	    errorOf: string;
	    atStart: boolean;
	    checked: number;
	    just: string;
	    failed: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.mode = source["mode"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.error = source["error"];
	        this.errorOf = source["errorOf"];
	        this.atStart = source["atStart"];
	        this.checked = source["checked"];
	        this.just = source["just"];
	        this.failed = source["failed"];
	    }
	}

}

export namespace collect {
	
	export class Info {
	    photos: number;
	    extra: number;
	    loose: number;
	    own: number;
	    guides: number;
	    missing: number;
	    missingGuides: number;
	    bytes: number;
	    scanning: boolean;
	    folderName: string;
	    fonts: number;
	    fontsRestricted: string[];
	    missingFonts: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photos = source["photos"];
	        this.extra = source["extra"];
	        this.loose = source["loose"];
	        this.own = source["own"];
	        this.guides = source["guides"];
	        this.missing = source["missing"];
	        this.missingGuides = source["missingGuides"];
	        this.bytes = source["bytes"];
	        this.scanning = source["scanning"];
	        this.folderName = source["folderName"];
	        this.fonts = source["fonts"];
	        this.fontsRestricted = source["fontsRestricted"];
	        this.missingFonts = source["missingFonts"];
	    }
	}

}

export namespace keymap {
	
	export class Entry {
	    id: string;
	    label: string;
	    short: string;
	    hint?: string;
	    group: string;
	    scope: string;
	    feature?: string;
	    defaults: string[];
	    fixed?: boolean;
	    repeat?: boolean;
	    dialogs?: boolean;
	    keys: string[];
	    custom: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.short = source["short"];
	        this.hint = source["hint"];
	        this.group = source["group"];
	        this.scope = source["scope"];
	        this.feature = source["feature"];
	        this.defaults = source["defaults"];
	        this.fixed = source["fixed"];
	        this.repeat = source["repeat"];
	        this.dialogs = source["dialogs"];
	        this.keys = source["keys"];
	        this.custom = source["custom"];
	    }
	}
	export class Named {
	    id: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new Named(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	    }
	}
	export class Ref {
	    id: string;
	    label: string;
	    chord: string;
	
	    static createFrom(source: any = {}) {
	        return new Ref(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.chord = source["chord"];
	    }
	}
	export class Reserved {
	    chord: string;
	    anyMods?: boolean;
	    prefix?: boolean;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new Reserved(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chord = source["chord"];
	        this.anyMods = source["anyMods"];
	        this.prefix = source["prefix"];
	        this.reason = source["reason"];
	    }
	}
	export class ScopeInfo {
	    id: string;
	    title: string;
	    rank: number;
	
	    static createFrom(source: any = {}) {
	        return new ScopeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.rank = source["rank"];
	    }
	}
	export class View {
	    commands: Entry[];
	    groups: Named[];
	    scopes: ScopeInfo[];
	    locked: Reserved[];
	    custom: Record<string, Array<string>>;
	    changed: number;
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commands = this.convertValues(source["commands"], Entry);
	        this.groups = this.convertValues(source["groups"], Named);
	        this.scopes = this.convertValues(source["scopes"], ScopeInfo);
	        this.locked = this.convertValues(source["locked"], Reserved);
	        this.custom = source["custom"];
	        this.changed = source["changed"];
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
	export class SetResult {
	    view: View;
	    conflict?: Ref;
	    replaced?: Ref;
	    shadows: Ref[];
	
	    static createFrom(source: any = {}) {
	        return new SetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], View);
	        this.conflict = this.convertValues(source["conflict"], Ref);
	        this.replaced = this.convertValues(source["replaced"], Ref);
	        this.shadows = this.convertValues(source["shadows"], Ref);
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

export namespace layout {
	
	export class Rect {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new Rect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}

}

export namespace library {
	
	export class Marks {
	    rating?: number;
	    label?: string;
	    labelText?: string;
	    flag?: number;
	
	    static createFrom(source: any = {}) {
	        return new Marks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rating = source["rating"];
	        this.label = source["label"];
	        this.labelText = source["labelText"];
	        this.flag = source["flag"];
	    }
	}
	export class Photo {
	    id: string;
	    path: string;
	    name: string;
	    w: number;
	    h: number;
	    shot?: number;
	    size?: number;
	    color?: string;
	    profile?: string;
	    xmp?: Marks;
	
	    static createFrom(source: any = {}) {
	        return new Photo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.shot = source["shot"];
	        this.size = source["size"];
	        this.color = source["color"];
	        this.profile = source["profile"];
	        this.xmp = this.convertValues(source["xmp"], Marks);
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
	export class ScanProgress {
	    active: boolean;
	    walking: boolean;
	    found: number;
	    read: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.walking = source["walking"];
	        this.found = source["found"];
	        this.read = source["read"];
	    }
	}

}

export namespace model {
	
	export class AlbumBgView {
	    name: string;
	    lib?: string;
	    fade: number;
	    pages: boolean;
	    preset?: string;
	    photo?: string;
	
	    static createFrom(source: any = {}) {
	        return new AlbumBgView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.lib = source["lib"];
	        this.fade = source["fade"];
	        this.pages = source["pages"];
	        this.preset = source["preset"];
	        this.photo = source["photo"];
	    }
	}
	export class AlbumPlan {
	    spreads: number;
	    photos: number;
	    free: number;
	    manual: number;
	    vignettes: number;
	    decor: number;
	    blank: number;
	
	    static createFrom(source: any = {}) {
	        return new AlbumPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spreads = source["spreads"];
	        this.photos = source["photos"];
	        this.free = source["free"];
	        this.manual = source["manual"];
	        this.vignettes = source["vignettes"];
	        this.decor = source["decor"];
	        this.blank = source["blank"];
	    }
	}
	export class Art {
	    kind: string;
	    inks?: string[];
	    seed?: number;
	
	    static createFrom(source: any = {}) {
	        return new Art(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.inks = source["inks"];
	        this.seed = source["seed"];
	    }
	}
	export class AutoOpts {
	    spreads: number;
	    maxPer: number;
	    minPer: number;
	    auto: boolean;
	    series: boolean;
	    variety: number;
	    families: string[];
	    similar: number;
	    keep: string[];
	    gray: number;
	    variant: number;
	    again: boolean;
	    album: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AutoOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spreads = source["spreads"];
	        this.maxPer = source["maxPer"];
	        this.minPer = source["minPer"];
	        this.auto = source["auto"];
	        this.series = source["series"];
	        this.variety = source["variety"];
	        this.families = source["families"];
	        this.similar = source["similar"];
	        this.keep = source["keep"];
	        this.gray = source["gray"];
	        this.variant = source["variant"];
	        this.again = source["again"];
	        this.album = source["album"];
	    }
	}
	export class AutoPlan {
	    photos: number;
	    spreads: number;
	    minSpreads: number;
	    maxSpreads: number;
	    avg: number;
	    min: number;
	    max: number;
	    reuse: number;
	    from: number;
	    to: number;
	    cuts: number;
	    similar?: number;
	    gray?: number;
	    album?: AlbumPlan;
	
	    static createFrom(source: any = {}) {
	        return new AutoPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photos = source["photos"];
	        this.spreads = source["spreads"];
	        this.minSpreads = source["minSpreads"];
	        this.maxSpreads = source["maxSpreads"];
	        this.avg = source["avg"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.reuse = source["reuse"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.cuts = source["cuts"];
	        this.similar = source["similar"];
	        this.gray = source["gray"];
	        this.album = this.convertValues(source["album"], AlbumPlan);
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
	export class PreviewSpread {
	    key: string;
	    rects: layout.Rect[];
	    family: string;
	    twin: number;
	    formName: string;
	    crop: number;
	    photos: number;
	    fallback: boolean;
	    gray: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewSpread(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.rects = this.convertValues(source["rects"], layout.Rect);
	        this.family = source["family"];
	        this.twin = source["twin"];
	        this.formName = source["formName"];
	        this.crop = source["crop"];
	        this.photos = source["photos"];
	        this.fallback = source["fallback"];
	        this.gray = source["gray"];
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
	export class Choice {
	    key: string;
	    rects: layout.Rect[];
	    family: string;
	    twin: number;
	    formName: string;
	    crop: number;
	
	    static createFrom(source: any = {}) {
	        return new Choice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.rects = this.convertValues(source["rects"], layout.Rect);
	        this.family = source["family"];
	        this.twin = source["twin"];
	        this.formName = source["formName"];
	        this.crop = source["crop"];
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
	export class SampleRow {
	    photos: number;
	    group: number;
	    inOrder: Choice[];
	    extra: Choice[];
	
	    static createFrom(source: any = {}) {
	        return new SampleRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photos = source["photos"];
	        this.group = source["group"];
	        this.inOrder = this.convertValues(source["inOrder"], Choice);
	        this.extra = this.convertValues(source["extra"], Choice);
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
	export class AutoPreview {
	    w: number;
	    h: number;
	    samples: SampleRow[];
	    spreads: PreviewSpread[];
	    fallback: number;
	
	    static createFrom(source: any = {}) {
	        return new AutoPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.w = source["w"];
	        this.h = source["h"];
	        this.samples = this.convertValues(source["samples"], SampleRow);
	        this.spreads = this.convertValues(source["spreads"], PreviewSpread);
	        this.fallback = source["fallback"];
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
	export class BackdropBox {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    rot: number;
	
	    static createFrom(source: any = {}) {
	        return new BackdropBox(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.rot = source["rot"];
	    }
	}
	export class Frame {
	    s: number;
	    x: number;
	    y: number;
	    r?: number;
	
	    static createFrom(source: any = {}) {
	        return new Frame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.s = source["s"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.r = source["r"];
	    }
	}
	export class BackdropView {
	    scope: string;
	    photo: string;
	    area: layout.Rect;
	    box: BackdropBox;
	    img: layout.Rect;
	    frame: Frame;
	    flipH: boolean;
	    flipV: boolean;
	    dpi: number;
	    fade: number;
	    lib?: string;
	    album?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BackdropView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.photo = source["photo"];
	        this.area = this.convertValues(source["area"], layout.Rect);
	        this.box = this.convertValues(source["box"], BackdropBox);
	        this.img = this.convertValues(source["img"], layout.Rect);
	        this.frame = this.convertValues(source["frame"], Frame);
	        this.flipH = source["flipH"];
	        this.flipV = source["flipV"];
	        this.dpi = source["dpi"];
	        this.fade = source["fade"];
	        this.lib = source["lib"];
	        this.album = source["album"];
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
	export class BgPlan {
	    spreads: number;
	    photos: number;
	
	    static createFrom(source: any = {}) {
	        return new BgPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spreads = source["spreads"];
	        this.photos = source["photos"];
	    }
	}
	export class Bin {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Bin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Border {
	    kind?: string;
	    photo?: string;
	    inks?: string[];
	    w?: number;
	    scale?: number;
	    cut: number[];
	    open?: boolean;
	    preset?: string;
	
	    static createFrom(source: any = {}) {
	        return new Border(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.photo = source["photo"];
	        this.inks = source["inks"];
	        this.w = source["w"];
	        this.scale = source["scale"];
	        this.cut = source["cut"];
	        this.open = source["open"];
	        this.preset = source["preset"];
	    }
	}
	export class BorderPatch {
	    inks?: string[];
	    width?: number;
	    scale?: number;
	
	    static createFrom(source: any = {}) {
	        return new BorderPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inks = source["inks"];
	        this.width = source["width"];
	        this.scale = source["scale"];
	    }
	}
	export class BorderView {
	    kind: string;
	    photo: string;
	    lib: string;
	    preset: string;
	    name: string;
	    inks: string[];
	    width: number;
	    defWidth?: number;
	    scale: number;
	    ins: number[];
	    url: string;
	    maskUrl: string;
	    pic: boolean;
	    open: boolean;
	    known: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BorderView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.photo = source["photo"];
	        this.lib = source["lib"];
	        this.preset = source["preset"];
	        this.name = source["name"];
	        this.inks = source["inks"];
	        this.width = source["width"];
	        this.defWidth = source["defWidth"];
	        this.scale = source["scale"];
	        this.ins = source["ins"];
	        this.url = source["url"];
	        this.maskUrl = source["maskUrl"];
	        this.pic = source["pic"];
	        this.open = source["open"];
	        this.known = source["known"];
	    }
	}
	export class Box {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    rot?: number;
	    shape?: string;
	    round?: number;
	
	    static createFrom(source: any = {}) {
	        return new Box(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.rot = source["rot"];
	        this.shape = source["shape"];
	        this.round = source["round"];
	    }
	}
	export class CaptionView {
	    photo: string;
	    id: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    text: string;
	    style: text.Style;
	    lines: text.Line[];
	    rot: number;
	    auto: boolean;
	    src?: string;
	    z: number;
	    draft?: boolean;
	    over: boolean;
	    overflow: number;
	    sheet: boolean;
	    pad: number;
	    who?: string[];
	
	    static createFrom(source: any = {}) {
	        return new CaptionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photo = source["photo"];
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.text = source["text"];
	        this.style = this.convertValues(source["style"], text.Style);
	        this.lines = this.convertValues(source["lines"], text.Line);
	        this.rot = source["rot"];
	        this.auto = source["auto"];
	        this.src = source["src"];
	        this.z = source["z"];
	        this.draft = source["draft"];
	        this.over = source["over"];
	        this.overflow = source["overflow"];
	        this.sheet = source["sheet"];
	        this.pad = source["pad"];
	        this.who = source["who"];
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
	
	export class ChoiceRows {
	    inOrder: Choice[];
	    rearranged: Choice[];
	    block: string;
	    more: boolean;
	    extra: Choice[];
	
	    static createFrom(source: any = {}) {
	        return new ChoiceRows(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inOrder = this.convertValues(source["inOrder"], Choice);
	        this.rearranged = this.convertValues(source["rearranged"], Choice);
	        this.block = source["block"];
	        this.more = source["more"];
	        this.extra = this.convertValues(source["extra"], Choice);
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
	export class Choices {
	    spread: string;
	    rev: number;
	    whole: ChoiceRows;
	    left?: ChoiceRows;
	    right?: ChoiceRows;
	
	    static createFrom(source: any = {}) {
	        return new Choices(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spread = source["spread"];
	        this.rev = source["rev"];
	        this.whole = this.convertValues(source["whole"], ChoiceRows);
	        this.left = this.convertValues(source["left"], ChoiceRows);
	        this.right = this.convertValues(source["right"], ChoiceRows);
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
	export class ClipInfo {
	    photos: number;
	    places: number;
	    texts: number;
	    stickers: number;
	    sheet: string;
	
	    static createFrom(source: any = {}) {
	        return new ClipInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photos = source["photos"];
	        this.places = source["places"];
	        this.texts = source["texts"];
	        this.stickers = source["stickers"];
	        this.sheet = source["sheet"];
	    }
	}
	export class ClipSel {
	    refs: string[];
	    texts: string[];
	    stickers: string[];
	
	    static createFrom(source: any = {}) {
	        return new ClipSel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.refs = source["refs"];
	        this.texts = source["texts"];
	        this.stickers = source["stickers"];
	    }
	}
	export class GridChoice {
	    id: string;
	    name: string;
	    scope: string;
	    boxes: Box[];
	    drop: number;
	
	    static createFrom(source: any = {}) {
	        return new GridChoice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scope = source["scope"];
	        this.boxes = this.convertValues(source["boxes"], Box);
	        this.drop = source["drop"];
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
	export class CoverGridChoices {
	    scope: string;
	    n: number;
	    box: layout.Rect;
	    places: number;
	    photos: number;
	    byPhoto: GridChoice[];
	    builtin: GridChoice[];
	    mine: GridChoice[];
	
	    static createFrom(source: any = {}) {
	        return new CoverGridChoices(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.n = source["n"];
	        this.box = this.convertValues(source["box"], layout.Rect);
	        this.places = source["places"];
	        this.photos = source["photos"];
	        this.byPhoto = this.convertValues(source["byPhoto"], GridChoice);
	        this.builtin = this.convertValues(source["builtin"], GridChoice);
	        this.mine = this.convertValues(source["mine"], GridChoice);
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
	export class SpineStep {
	    upto: number;
	    mm: number;
	
	    static createFrom(source: any = {}) {
	        return new SpineStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upto = source["upto"];
	        this.mm = source["mm"];
	    }
	}
	export class CoverSpec {
	    preset: string;
	    w?: number;
	    h?: number;
	    wrap: number;
	    hinge?: number;
	    steps?: SpineStep[];
	    a?: number;
	    b?: number;
	    safe?: number;
	    spineSafe?: number;
	
	    static createFrom(source: any = {}) {
	        return new CoverSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preset = source["preset"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.wrap = source["wrap"];
	        this.hinge = source["hinge"];
	        this.steps = this.convertValues(source["steps"], SpineStep);
	        this.a = source["a"];
	        this.b = source["b"];
	        this.safe = source["safe"];
	        this.spineSafe = source["spineSafe"];
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
	export class CoverPreset {
	    id: string;
	    name: string;
	    group: string;
	    spec: CoverSpec;
	
	    static createFrom(source: any = {}) {
	        return new CoverPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.spec = this.convertValues(source["spec"], CoverSpec);
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
	export class CoverScope {
	    scope: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new CoverScope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.label = source["label"];
	    }
	}
	export class CoverSide {
	    scope: string;
	    label: string;
	    box: layout.Rect;
	    places: number;
	    known: boolean;
	    alone: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CoverSide(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.label = source["label"];
	        this.box = this.convertValues(source["box"], layout.Rect);
	        this.places = source["places"];
	        this.known = source["known"];
	        this.alone = source["alone"];
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
	
	export class GuideLine {
	    v: boolean;
	    at: number;
	
	    static createFrom(source: any = {}) {
	        return new GuideLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.v = source["v"];
	        this.at = source["at"];
	    }
	}
	export class GuideView {
	    name: string;
	    key: string;
	    ok: boolean;
	    image: boolean;
	    warn?: string;
	    note?: string;
	    lines?: GuideLine[];
	
	    static createFrom(source: any = {}) {
	        return new GuideView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.key = source["key"];
	        this.ok = source["ok"];
	        this.image = source["image"];
	        this.warn = source["warn"];
	        this.note = source["note"];
	        this.lines = this.convertValues(source["lines"], GuideLine);
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
	export class CoverZone {
	    kind: string;
	    rect: layout.Rect;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new CoverZone(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.rect = this.convertValues(source["rect"], layout.Rect);
	        this.label = source["label"];
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
	export class Vars {
	    title?: string;
	    subtitle?: string;
	    year?: string;
	    lead?: string;
	
	    static createFrom(source: any = {}) {
	        return new Vars(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.year = source["year"];
	        this.lead = source["lead"];
	    }
	}
	export class VigShape {
	    name: string;
	    boxes: Box[];
	    roles: string[];
	    head?: Box;
	    pages: number;
	    aspect: number;
	    ratio: string;
	    w?: number;
	    h?: number;
	
	    static createFrom(source: any = {}) {
	        return new VigShape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.boxes = this.convertValues(source["boxes"], Box);
	        this.roles = source["roles"];
	        this.head = this.convertValues(source["head"], Box);
	        this.pages = source["pages"];
	        this.aspect = source["aspect"];
	        this.ratio = source["ratio"];
	        this.w = source["w"];
	        this.h = source["h"];
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
	export class VignetteView {
	    tpl: string;
	    teachers: number;
	    pupils: number;
	    ratio?: string;
	    ttext?: string;
	    head?: string;
	    own?: VigShape;
	    shape?: string;
	    round?: number;
	    places: number;
	    max: number;
	    group: boolean;
	    ownName?: string;
	    tplName: string;
	    manual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VignetteView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tpl = source["tpl"];
	        this.teachers = source["teachers"];
	        this.pupils = source["pupils"];
	        this.ratio = source["ratio"];
	        this.ttext = source["ttext"];
	        this.head = source["head"];
	        this.own = this.convertValues(source["own"], VigShape);
	        this.shape = source["shape"];
	        this.round = source["round"];
	        this.places = source["places"];
	        this.max = source["max"];
	        this.group = source["group"];
	        this.ownName = source["ownName"];
	        this.tplName = source["tplName"];
	        this.manual = source["manual"];
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
	export class SheetCaps {
	    style: text.Style;
	    pad: number;
	    over?: boolean;
	    wrap?: boolean;
	    text?: string;
	
	    static createFrom(source: any = {}) {
	        return new SheetCaps(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.style = this.convertValues(source["style"], text.Style);
	        this.pad = source["pad"];
	        this.over = source["over"];
	        this.wrap = source["wrap"];
	        this.text = source["text"];
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
	export class StubView {
	    place: string;
	    photo: string;
	    id: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    text: string;
	    style: text.Style;
	    lines: text.Line[];
	    rot: number;
	    auto: boolean;
	    src?: string;
	    z: number;
	    draft?: boolean;
	    who?: string[];
	
	    static createFrom(source: any = {}) {
	        return new StubView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.place = source["place"];
	        this.photo = source["photo"];
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.text = source["text"];
	        this.style = this.convertValues(source["style"], text.Style);
	        this.lines = this.convertValues(source["lines"], text.Line);
	        this.rot = source["rot"];
	        this.auto = source["auto"];
	        this.src = source["src"];
	        this.z = source["z"];
	        this.draft = source["draft"];
	        this.who = source["who"];
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
	export class StickerView {
	    id: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    rot: number;
	    flipH: boolean;
	    flipV: boolean;
	    fade: number;
	    z: number;
	    photo: string;
	    lib?: string;
	    art?: Art;
	    name: string;
	    inks: number;
	    url: string;
	    gen: boolean;
	    stretch: boolean;
	    aspect: number;
	    dpi: number;
	    known: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StickerView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.rot = source["rot"];
	        this.flipH = source["flipH"];
	        this.flipV = source["flipV"];
	        this.fade = source["fade"];
	        this.z = source["z"];
	        this.photo = source["photo"];
	        this.lib = source["lib"];
	        this.art = this.convertValues(source["art"], Art);
	        this.name = source["name"];
	        this.inks = source["inks"];
	        this.url = source["url"];
	        this.gen = source["gen"];
	        this.stretch = source["stretch"];
	        this.aspect = source["aspect"];
	        this.dpi = source["dpi"];
	        this.known = source["known"];
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
	export class Pattern {
	    kind: string;
	    inks?: string[];
	    scale?: number;
	    seed?: number;
	    fade?: number;
	    preset?: string;
	
	    static createFrom(source: any = {}) {
	        return new Pattern(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.inks = source["inks"];
	        this.scale = source["scale"];
	        this.seed = source["seed"];
	        this.fade = source["fade"];
	        this.preset = source["preset"];
	    }
	}
	export class PatternView {
	    scope: string;
	    area: layout.Rect;
	    pat: Pattern;
	    name: string;
	    inks: number;
	    random: boolean;
	    known: boolean;
	    url: string;
	    preset?: string;
	    album?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PatternView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.area = this.convertValues(source["area"], layout.Rect);
	        this.pat = this.convertValues(source["pat"], Pattern);
	        this.name = source["name"];
	        this.inks = source["inks"];
	        this.random = source["random"];
	        this.known = source["known"];
	        this.url = source["url"];
	        this.preset = source["preset"];
	        this.album = source["album"];
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
	export class PlaceView {
	    id: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    photo: string;
	    fit: string;
	    spine: boolean;
	    rot: number;
	    z: number;
	    role: string;
	    shape: string;
	    round: number;
	    radius: number;
	    border?: BorderView;
	
	    static createFrom(source: any = {}) {
	        return new PlaceView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.photo = source["photo"];
	        this.fit = source["fit"];
	        this.spine = source["spine"];
	        this.rot = source["rot"];
	        this.z = source["z"];
	        this.role = source["role"];
	        this.shape = source["shape"];
	        this.round = source["round"];
	        this.radius = source["radius"];
	        this.border = this.convertValues(source["border"], BorderView);
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
	export class TextView {
	    id: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    text: string;
	    style: text.Style;
	    lines: text.Line[];
	    rot: number;
	    auto: boolean;
	    src?: string;
	    z: number;
	    draft?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TextView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.text = source["text"];
	        this.style = this.convertValues(source["style"], text.Style);
	        this.lines = this.convertValues(source["lines"], text.Line);
	        this.rot = source["rot"];
	        this.auto = source["auto"];
	        this.src = source["src"];
	        this.z = source["z"];
	        this.draft = source["draft"];
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
	export class FrameView {
	    img: layout.Rect;
	    s: number;
	    x: number;
	    y: number;
	    r: number;
	    turn: number;
	    tilt: number;
	    dpi: number;
	    flipH: boolean;
	    flipV: boolean;
	    gray: boolean;
	    bright: number;
	    contrast: number;
	    filter: string;
	    cellRot: number;
	    z: number;
	    shape: string;
	    radius: number;
	    win: layout.Rect;
	    winShape: string;
	    winRadius: number;
	    fade: number;
	    own?: number;
	
	    static createFrom(source: any = {}) {
	        return new FrameView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.img = this.convertValues(source["img"], layout.Rect);
	        this.s = source["s"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.r = source["r"];
	        this.turn = source["turn"];
	        this.tilt = source["tilt"];
	        this.dpi = source["dpi"];
	        this.flipH = source["flipH"];
	        this.flipV = source["flipV"];
	        this.gray = source["gray"];
	        this.bright = source["bright"];
	        this.contrast = source["contrast"];
	        this.filter = source["filter"];
	        this.cellRot = source["cellRot"];
	        this.z = source["z"];
	        this.shape = source["shape"];
	        this.radius = source["radius"];
	        this.win = this.convertValues(source["win"], layout.Rect);
	        this.winShape = source["winShape"];
	        this.winRadius = source["winRadius"];
	        this.fade = source["fade"];
	        this.own = source["own"];
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
	export class PageView {
	    photos: number;
	    variant: number;
	    variants: number;
	    rearranged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PageView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photos = source["photos"];
	        this.variant = source["variant"];
	        this.variants = source["variants"];
	        this.rearranged = source["rearranged"];
	    }
	}
	export class SpreadView {
	    id: string;
	    photos: string[];
	    w: number;
	    h: number;
	    mid: number;
	    rects: layout.Rect[];
	    layout: string;
	    variant: number;
	    variants: number;
	    more: boolean;
	    rearranged: boolean;
	    pages: PageView[];
	    frames: FrameView[];
	    texts: TextView[];
	    captions: CaptionView[];
	    bg: string[];
	    bgSet: string[];
	    grid: boolean;
	    places: PlaceView[];
	    name: string;
	    backdrops: BackdropView[];
	    patterns: PatternView[];
	    stickers: StickerView[];
	    lowDPI: number;
	    stubs: StubView[];
	    caps?: SheetCaps;
	    kind: string;
	    vig?: VignetteView;
	    vars: Vars;
	    fields: string[];
	    noAlbumBg?: boolean;
	    foldHidden?: number;
	    lookStep?: number;
	
	    static createFrom(source: any = {}) {
	        return new SpreadView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.photos = source["photos"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.mid = source["mid"];
	        this.rects = this.convertValues(source["rects"], layout.Rect);
	        this.layout = source["layout"];
	        this.variant = source["variant"];
	        this.variants = source["variants"];
	        this.more = source["more"];
	        this.rearranged = source["rearranged"];
	        this.pages = this.convertValues(source["pages"], PageView);
	        this.frames = this.convertValues(source["frames"], FrameView);
	        this.texts = this.convertValues(source["texts"], TextView);
	        this.captions = this.convertValues(source["captions"], CaptionView);
	        this.bg = source["bg"];
	        this.bgSet = source["bgSet"];
	        this.grid = source["grid"];
	        this.places = this.convertValues(source["places"], PlaceView);
	        this.name = source["name"];
	        this.backdrops = this.convertValues(source["backdrops"], BackdropView);
	        this.patterns = this.convertValues(source["patterns"], PatternView);
	        this.stickers = this.convertValues(source["stickers"], StickerView);
	        this.lowDPI = source["lowDPI"];
	        this.stubs = this.convertValues(source["stubs"], StubView);
	        this.caps = this.convertValues(source["caps"], SheetCaps);
	        this.kind = source["kind"];
	        this.vig = this.convertValues(source["vig"], VignetteView);
	        this.vars = this.convertValues(source["vars"], Vars);
	        this.fields = source["fields"];
	        this.noAlbumBg = source["noAlbumBg"];
	        this.foldHidden = source["foldHidden"];
	        this.lookStep = source["lookStep"];
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
	export class CoverView {
	    sheet: SpreadView;
	    spec: CoverSpec;
	    preset: string;
	    spine: number;
	    n: number;
	    over: boolean;
	    layout: string;
	    spineBg: string;
	    spineText: string;
	    zones: CoverZone[];
	    drop: CoverZone[];
	    scopes: CoverScope[];
	    safe: layout.Rect[];
	    textMaxW: number;
	    sides: CoverSide[];
	    markup?: GuideView;
	    pxW: number;
	    pxH: number;
	    fix: string;
	    missing: string;
	
	    static createFrom(source: any = {}) {
	        return new CoverView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sheet = this.convertValues(source["sheet"], SpreadView);
	        this.spec = this.convertValues(source["spec"], CoverSpec);
	        this.preset = source["preset"];
	        this.spine = source["spine"];
	        this.n = source["n"];
	        this.over = source["over"];
	        this.layout = source["layout"];
	        this.spineBg = source["spineBg"];
	        this.spineText = source["spineText"];
	        this.zones = this.convertValues(source["zones"], CoverZone);
	        this.drop = this.convertValues(source["drop"], CoverZone);
	        this.scopes = this.convertValues(source["scopes"], CoverScope);
	        this.safe = this.convertValues(source["safe"], layout.Rect);
	        this.textMaxW = source["textMaxW"];
	        this.sides = this.convertValues(source["sides"], CoverSide);
	        this.markup = this.convertValues(source["markup"], GuideView);
	        this.pxW = source["pxW"];
	        this.pxH = source["pxH"];
	        this.fix = source["fix"];
	        this.missing = source["missing"];
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
	
	export class FaceGuide {
	    on: boolean;
	    w: number;
	    h: number;
	    eyeY: number;
	    path: number[][];
	
	    static createFrom(source: any = {}) {
	        return new FaceGuide(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.on = source["on"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.eyeY = source["eyeY"];
	        this.path = source["path"];
	    }
	}
	export class FaceReport {
	    fitted: number;
	    noFace: number;
	    many: number;
	    missing: number;
	    small: number;
	    big: number;
	    edge: number;
	    contain: number;
	    manual: number;
	    changed: number;
	    check: string[];
	    short: string[];
	    asked: boolean;
	    refErr?: string;
	
	    static createFrom(source: any = {}) {
	        return new FaceReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fitted = source["fitted"];
	        this.noFace = source["noFace"];
	        this.many = source["many"];
	        this.missing = source["missing"];
	        this.small = source["small"];
	        this.big = source["big"];
	        this.edge = source["edge"];
	        this.contain = source["contain"];
	        this.manual = source["manual"];
	        this.changed = source["changed"];
	        this.check = source["check"];
	        this.short = source["short"];
	        this.asked = source["asked"];
	        this.refErr = source["refErr"];
	    }
	}
	export class FontUse {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new FontUse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	
	
	
	export class GridChoices {
	    n: number;
	    builtin: GridChoice[];
	    mine: GridChoice[];
	
	    static createFrom(source: any = {}) {
	        return new GridChoices(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.n = source["n"];
	        this.builtin = this.convertValues(source["builtin"], GridChoice);
	        this.mine = this.convertValues(source["mine"], GridChoice);
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
	export class GridPlan {
	    frozen: number;
	    left: number;
	    texts: number;
	    stickers: number;
	
	    static createFrom(source: any = {}) {
	        return new GridPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.frozen = source["frozen"];
	        this.left = source["left"];
	        this.texts = source["texts"];
	        this.stickers = source["stickers"];
	    }
	}
	export class GridTarget {
	    spread: string;
	    scope: string;
	
	    static createFrom(source: any = {}) {
	        return new GridTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spread = source["spread"];
	        this.scope = source["scope"];
	    }
	}
	
	
	export class LookOpts {
	    mode: string;
	    spreads: number;
	
	    static createFrom(source: any = {}) {
	        return new LookOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.spreads = source["spreads"];
	    }
	}
	export class LookPlan {
	    mode: string;
	    trial: boolean;
	    touched: number;
	    full: boolean;
	    spreads: number;
	    places: number;
	    drafts: number;
	    replaced: number;
	    coverTouched: boolean;
	    coverReplaced: boolean;
	    anchors: number;
	    cover: boolean;
	    left: number;
	    prev: string;
	    missing: number;
	
	    static createFrom(source: any = {}) {
	        return new LookPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.trial = source["trial"];
	        this.touched = source["touched"];
	        this.full = source["full"];
	        this.spreads = source["spreads"];
	        this.places = source["places"];
	        this.drafts = source["drafts"];
	        this.replaced = source["replaced"];
	        this.coverTouched = source["coverTouched"];
	        this.coverReplaced = source["coverReplaced"];
	        this.anchors = source["anchors"];
	        this.cover = source["cover"];
	        this.left = source["left"];
	        this.prev = source["prev"];
	        this.missing = source["missing"];
	    }
	}
	export class LookView {
	    key: string;
	    name: string;
	    found: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LookView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.found = source["found"];
	    }
	}
	export class LooseFileView {
	    path: string;
	    photo: string;
	
	    static createFrom(source: any = {}) {
	        return new LooseFileView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.photo = source["photo"];
	    }
	}
	export class LooseView {
	    id: string;
	    at: number;
	    files: LooseFileView[];
	
	    static createFrom(source: any = {}) {
	        return new LooseView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.at = source["at"];
	        this.files = this.convertValues(source["files"], LooseFileView);
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
	export class MarkPatch {
	    rating?: number;
	    step?: number;
	    label?: string;
	    toggle?: boolean;
	    flag?: number;
	
	    static createFrom(source: any = {}) {
	        return new MarkPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rating = source["rating"];
	        this.step = source["step"];
	        this.label = source["label"];
	        this.toggle = source["toggle"];
	        this.flag = source["flag"];
	    }
	}
	export class Person {
	    last?: string;
	    first?: string;
	    middle?: string;
	    subj?: string;
	
	    static createFrom(source: any = {}) {
	        return new Person(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last = source["last"];
	        this.first = source["first"];
	        this.middle = source["middle"];
	        this.subj = source["subj"];
	    }
	}
	export class NamesLeft {
	    line: number;
	    who: Person;
	    reason: string;
	    file: string;
	
	    static createFrom(source: any = {}) {
	        return new NamesLeft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.who = this.convertValues(source["who"], Person);
	        this.reason = source["reason"];
	        this.file = source["file"];
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
	export class NamesOpts {
	    from: string;
	    text: string;
	    firstFirst: boolean;
	    caption: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NamesOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.text = source["text"];
	        this.firstFirst = source["firstFirst"];
	        this.caption = source["caption"];
	    }
	}
	export class NamesRow {
	    place: string;
	    photo: string;
	    seat: boolean;
	    tag: string;
	    no: number;
	    line: number;
	    who?: Person;
	
	    static createFrom(source: any = {}) {
	        return new NamesRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.place = source["place"];
	        this.photo = source["photo"];
	        this.seat = source["seat"];
	        this.tag = source["tag"];
	        this.no = source["no"];
	        this.line = source["line"];
	        this.who = this.convertValues(source["who"], Person);
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
	export class NamesPlan {
	    rows: NamesRow[];
	    left: NamesLeft[];
	    extra: number;
	    seats: number;
	    named: number;
	    missing: number;
	    cols: string[];
	    ignored: string[];
	    byFile: boolean;
	    order: boolean;
	    teachers: number;
	    pupils: number;
	    teacherSeats: number;
	    pupilSeats: number;
	    needCaption: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NamesPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], NamesRow);
	        this.left = this.convertValues(source["left"], NamesLeft);
	        this.extra = source["extra"];
	        this.seats = source["seats"];
	        this.named = source["named"];
	        this.missing = source["missing"];
	        this.cols = source["cols"];
	        this.ignored = source["ignored"];
	        this.byFile = source["byFile"];
	        this.order = source["order"];
	        this.teachers = source["teachers"];
	        this.pupils = source["pupils"];
	        this.teacherSeats = source["teacherSeats"];
	        this.pupilSeats = source["pupilSeats"];
	        this.needCaption = source["needCaption"];
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
	
	export class PageLimitView {
	    who: string;
	    range: string;
	    count: number;
	    min: number;
	    max: number;
	    warn: string;
	
	    static createFrom(source: any = {}) {
	        return new PageLimitView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.who = source["who"];
	        this.range = source["range"];
	        this.count = source["count"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.warn = source["warn"];
	    }
	}
	export class PagePreset {
	    label: string;
	    group: string;
	    w: number;
	    h: number;
	    dpi?: number;
	    layflat?: boolean;
	    safe?: number;
	    bleed?: number;
	    hint?: string;
	
	    static createFrom(source: any = {}) {
	        return new PagePreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.group = source["group"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.dpi = source["dpi"];
	        this.layflat = source["layflat"];
	        this.safe = source["safe"];
	        this.bleed = source["bleed"];
	        this.hint = source["hint"];
	    }
	}
	
	export class Pt {
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new Pt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class PasteReq {
	    sheet: string;
	    hover?: Pt;
	    at?: Pt;
	    repeat: string;
	
	    static createFrom(source: any = {}) {
	        return new PasteReq(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sheet = source["sheet"];
	        this.hover = this.convertValues(source["hover"], Pt);
	        this.at = this.convertValues(source["at"], Pt);
	        this.repeat = source["repeat"];
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
	
	
	
	export class PersonView {
	    last: string;
	    first: string;
	    middle: string;
	    subject: string;
	    fromFile: boolean;
	    teacher: boolean;
	    place: string;
	    seatNamed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PersonView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last = source["last"];
	        this.first = source["first"];
	        this.middle = source["middle"];
	        this.subject = source["subject"];
	        this.fromFile = source["fromFile"];
	        this.teacher = source["teacher"];
	        this.place = source["place"];
	        this.seatNamed = source["seatNamed"];
	    }
	}
	export class PhotoUse {
	    sheet: number;
	    label: string;
	    kind: string;
	    spread: string;
	    page: string;
	    what: string;
	    pick?: string;
	
	    static createFrom(source: any = {}) {
	        return new PhotoUse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sheet = source["sheet"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.spread = source["spread"];
	        this.page = source["page"];
	        this.what = source["what"];
	        this.pick = source["pick"];
	    }
	}
	export class PlaceBox {
	    id: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new PlaceBox(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	
	
	
	
	export class SaveLookOpts {
	    theme: string;
	    name: string;
	    ids: string[];
	    repFrom: number;
	    repTo: number;
	    places: boolean;
	    drafts: boolean;
	    cover: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveLookOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.name = source["name"];
	        this.ids = source["ids"];
	        this.repFrom = source["repFrom"];
	        this.repTo = source["repTo"];
	        this.places = source["places"];
	        this.drafts = source["drafts"];
	        this.cover = source["cover"];
	    }
	}
	export class Settings {
	    pageW: number;
	    pageH: number;
	    margin: number;
	    gap: number;
	    layflat: boolean;
	    foldAny: boolean;
	    dpi: number;
	    bg: string;
	    safe: number;
	    sheet?: boolean;
	    bleed?: number;
	    crop?: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pageW = source["pageW"];
	        this.pageH = source["pageH"];
	        this.margin = source["margin"];
	        this.gap = source["gap"];
	        this.layflat = source["layflat"];
	        this.foldAny = source["foldAny"];
	        this.dpi = source["dpi"];
	        this.bg = source["bg"];
	        this.safe = source["safe"];
	        this.sheet = source["sheet"];
	        this.bleed = source["bleed"];
	        this.crop = source["crop"];
	    }
	}
	
	export class SimilarDrop {
	    id: string;
	    like: string;
	    dist: number;
	
	    static createFrom(source: any = {}) {
	        return new SimilarDrop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.like = source["like"];
	        this.dist = source["dist"];
	    }
	}
	
	
	
	
	
	export class UsedAt {
	    id: string;
	    uses: PhotoUse[];
	    places: number;
	    move: string;
	
	    static createFrom(source: any = {}) {
	        return new UsedAt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uses = this.convertValues(source["uses"], PhotoUse);
	        this.places = source["places"];
	        this.move = source["move"];
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
	
	export class View {
	    product: string;
	    spreads: SpreadView[];
	    settings: Settings;
	    folders: string[];
	    loose: LooseView[];
	    path: string;
	    dirty: boolean;
	    canUndo: boolean;
	    canRedo: boolean;
	    rev: number;
	    maxSpreadPhotos: number;
	    safe: layout.Rect[];
	    bleed: layout.Rect[];
	    spreadMarkup?: GuideView;
	    minDPI: number;
	    lowDPI: number;
	    emptyPlaces: number;
	    fillFrom: number[];
	    coverFill: number;
	    textMaxW: number;
	    backdropUsed: string[];
	    libPhotos: string[];
	    albumFonts?: FontUse[];
	    marksRev: number;
	    bins: Bin[];
	    unfilled: string[];
	    unnamed: number;
	    drafts: number;
	    look?: LookView;
	    cover?: CoverView;
	    albumBg?: AlbumBgView;
	    pageLimit?: PageLimitView;
	    again: boolean;
	    seq: number;
	    base?: number;
	    order?: string[];
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.product = source["product"];
	        this.spreads = this.convertValues(source["spreads"], SpreadView);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.folders = source["folders"];
	        this.loose = this.convertValues(source["loose"], LooseView);
	        this.path = source["path"];
	        this.dirty = source["dirty"];
	        this.canUndo = source["canUndo"];
	        this.canRedo = source["canRedo"];
	        this.rev = source["rev"];
	        this.maxSpreadPhotos = source["maxSpreadPhotos"];
	        this.safe = this.convertValues(source["safe"], layout.Rect);
	        this.bleed = this.convertValues(source["bleed"], layout.Rect);
	        this.spreadMarkup = this.convertValues(source["spreadMarkup"], GuideView);
	        this.minDPI = source["minDPI"];
	        this.lowDPI = source["lowDPI"];
	        this.emptyPlaces = source["emptyPlaces"];
	        this.fillFrom = source["fillFrom"];
	        this.coverFill = source["coverFill"];
	        this.textMaxW = source["textMaxW"];
	        this.backdropUsed = source["backdropUsed"];
	        this.libPhotos = source["libPhotos"];
	        this.albumFonts = this.convertValues(source["albumFonts"], FontUse);
	        this.marksRev = source["marksRev"];
	        this.bins = this.convertValues(source["bins"], Bin);
	        this.unfilled = source["unfilled"];
	        this.unnamed = source["unnamed"];
	        this.drafts = source["drafts"];
	        this.look = this.convertValues(source["look"], LookView);
	        this.cover = this.convertValues(source["cover"], CoverView);
	        this.albumBg = this.convertValues(source["albumBg"], AlbumBgView);
	        this.pageLimit = this.convertValues(source["pageLimit"], PageLimitView);
	        this.again = source["again"];
	        this.seq = source["seq"];
	        this.base = source["base"];
	        this.order = source["order"];
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
	export class VigChoice {
	    id: string;
	    name: string;
	    boxes: Box[];
	    roles: string[];
	    pupilW: number;
	    small: boolean;
	    over: boolean;
	    own: boolean;
	    ratio: string;
	    gone: boolean;
	    shape?: string;
	    round?: number;
	    family: string;
	    head?: Box;
	
	    static createFrom(source: any = {}) {
	        return new VigChoice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.boxes = this.convertValues(source["boxes"], Box);
	        this.roles = source["roles"];
	        this.pupilW = source["pupilW"];
	        this.small = source["small"];
	        this.over = source["over"];
	        this.own = source["own"];
	        this.ratio = source["ratio"];
	        this.gone = source["gone"];
	        this.shape = source["shape"];
	        this.round = source["round"];
	        this.family = source["family"];
	        this.head = this.convertValues(source["head"], Box);
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
	export class VigPreview {
	    spread: SpreadView;
	    left: number;
	
	    static createFrom(source: any = {}) {
	        return new VigPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spread = this.convertValues(source["spread"], SpreadView);
	        this.left = source["left"];
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
	export class Vignette {
	    tpl: string;
	    teachers: number;
	    pupils: number;
	    ratio?: string;
	    ttext?: string;
	    head?: string;
	    own?: VigShape;
	    shape?: string;
	    round?: number;
	
	    static createFrom(source: any = {}) {
	        return new Vignette(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tpl = source["tpl"];
	        this.teachers = source["teachers"];
	        this.pupils = source["pupils"];
	        this.ratio = source["ratio"];
	        this.ttext = source["ttext"];
	        this.head = source["head"];
	        this.own = this.convertValues(source["own"], VigShape);
	        this.shape = source["shape"];
	        this.round = source["round"];
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
	export class VigSetup {
	    vig: Vignette;
	    vars: Vars;
	
	    static createFrom(source: any = {}) {
	        return new VigSetup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vig = this.convertValues(source["vig"], Vignette);
	        this.vars = this.convertValues(source["vars"], Vars);
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

export namespace prefs {
	
	export class CustomLevel {
	    base: string;
	    on: string[];
	    off: string[];
	
	    static createFrom(source: any = {}) {
	        return new CustomLevel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base = source["base"];
	        this.on = source["on"];
	        this.off = source["off"];
	    }
	}
	export class Prefs {
	    level: string;
	    showStart: boolean;
	    first: boolean;
	    custom?: CustomLevel;
	    tourDue: boolean;
	    tourMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Prefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.showStart = source["showStart"];
	        this.first = source["first"];
	        this.custom = this.convertValues(source["custom"], CustomLevel);
	        this.tourDue = source["tourDue"];
	        this.tourMore = source["tourMore"];
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
	export class RecentProject {
	    path: string;
	    name: string;
	    exists: boolean;
	    modified: number;
	
	    static createFrom(source: any = {}) {
	        return new RecentProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.exists = source["exists"];
	        this.modified = source["modified"];
	    }
	}

}

export namespace printprof {
	
	export class InfoFormat {
	    name: string;
	    pageW: number;
	    pageH: number;
	    pxW?: number;
	    pxH?: number;
	
	    static createFrom(source: any = {}) {
	        return new InfoFormat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pageW = source["pageW"];
	        this.pageH = source["pageH"];
	        this.pxW = source["pxW"];
	        this.pxH = source["pxH"];
	    }
	}
	export class Info {
	    id: string;
	    name: string;
	    source: string;
	    version: number;
	    hasExport: boolean;
	    formats: InfoFormat[];
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.version = source["version"];
	        this.hasExport = source["hasExport"];
	        this.formats = this.convertValues(source["formats"], InfoFormat);
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

export namespace printshop {
	
	export class Plan {
	    kind: string;
	    title: string;
	    format: string;
	    off: string;
	    pages: string;
	    px: string;
	    default: boolean;
	    spreads: number;
	    spine: number;
	    min: number;
	    max: number;
	    folder: string;
	    coverName: string;
	    quality: number;
	    block: string[];
	    coverFix: boolean;
	    fixAt: number;
	    warn: string[];
	    memo: string[];
	    done: string;
	    site: string;
	    siteButton: string;
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.format = source["format"];
	        this.off = source["off"];
	        this.pages = source["pages"];
	        this.px = source["px"];
	        this.default = source["default"];
	        this.spreads = source["spreads"];
	        this.spine = source["spine"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.folder = source["folder"];
	        this.coverName = source["coverName"];
	        this.quality = source["quality"];
	        this.block = source["block"];
	        this.coverFix = source["coverFix"];
	        this.fixAt = source["fixAt"];
	        this.warn = source["warn"];
	        this.memo = source["memo"];
	        this.done = source["done"];
	        this.site = source["site"];
	        this.siteButton = source["siteButton"];
	    }
	}

}

export namespace text {
	
	export class Face {
	    file: string;
	    bold: boolean;
	    italic: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Face(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.bold = source["bold"];
	        this.italic = source["italic"];
	    }
	}
	export class Family {
	    id: string;
	    name: string;
	    css: string;
	    group: string;
	    faces: Face[];
	    hidden?: boolean;
	    noCyr?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Family(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.css = source["css"];
	        this.group = source["group"];
	        this.faces = this.convertValues(source["faces"], Face);
	        this.hidden = source["hidden"];
	        this.noCyr = source["noCyr"];
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
	export class Line {
	    text: string;
	    x: number;
	    y: number;
	    w: number;
	
	    static createFrom(source: any = {}) {
	        return new Line(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	    }
	}
	export class Style {
	    font: string;
	    size: number;
	    color: string;
	    align: string;
	    bold?: boolean;
	    italic?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Style(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.font = source["font"];
	        this.size = source["size"];
	        this.color = source["color"];
	        this.align = source["align"];
	        this.bold = source["bold"];
	        this.italic = source["italic"];
	    }
	}

}

