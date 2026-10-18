export namespace app {
	
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
	    readOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DecorCatalog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.themes = this.convertValues(source["themes"], DecorTheme);
	        this.items = this.convertValues(source["items"], DecorItem);
	        this.recent = source["recent"];
	        this.readOnly = source["readOnly"];
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
	    fsfera: boolean;
	
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
	        this.fsfera = source["fsfera"];
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
	export class State {
	    view: model.View;
	    photos: library.Photo[];
	    scan: library.ScanProgress;
	    notice?: string;
	    missing: string[];
	    version: string;
	    fonts: text.Family[];
	    prefs: prefs.Prefs;
	    recent: prefs.RecentProject[];
	    cancelled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
	        this.photos = this.convertValues(source["photos"], library.Photo);
	        this.scan = this.convertValues(source["scan"], library.ScanProgress);
	        this.notice = source["notice"];
	        this.missing = source["missing"];
	        this.version = source["version"];
	        this.fonts = this.convertValues(source["fonts"], text.Family);
	        this.prefs = this.convertValues(source["prefs"], prefs.Prefs);
	        this.recent = this.convertValues(source["recent"], prefs.RecentProject);
	        this.cancelled = source["cancelled"];
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

}

export namespace fsfera {
	
	export class Plan {
	    format: string;
	    pages: string;
	    spreads: number;
	    spine: number;
	    max: number;
	    folder: string;
	    block: string[];
	    coverFix: boolean;
	    fixAt: number;
	    warn: string[];
	    memo: string[];
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.pages = source["pages"];
	        this.spreads = source["spreads"];
	        this.spine = source["spine"];
	        this.max = source["max"];
	        this.folder = source["folder"];
	        this.block = source["block"];
	        this.coverFix = source["coverFix"];
	        this.fixAt = source["fixAt"];
	        this.warn = source["warn"];
	        this.memo = source["memo"];
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
	    }
	}
	export class PreviewSpread {
	    key: string;
	    rects: layout.Rect[];
	    family: string;
	    twin: number;
	    formName: string;
	    photos: number;
	    fallback: boolean;
	
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
	        this.photos = source["photos"];
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
	export class Choice {
	    key: string;
	    rects: layout.Rect[];
	    family: string;
	    twin: number;
	    formName: string;
	
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
	    cellRot: number;
	    z: number;
	    shape: string;
	    radius: number;
	
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
	        this.cellRot = source["cellRot"];
	        this.z = source["z"];
	        this.shape = source["shape"];
	        this.radius = source["radius"];
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
	    markup?: GuideView;
	    pxW: number;
	    pxH: number;
	    fix: string;
	
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
	        this.markup = this.convertValues(source["markup"], GuideView);
	        this.pxW = source["pxW"];
	        this.pxH = source["pxH"];
	        this.fix = source["fix"];
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
	    photo: string;
	    tag: string;
	    no: number;
	    line: number;
	    who?: Person;
	
	    static createFrom(source: any = {}) {
	        return new NamesRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photo = source["photo"];
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
	
	
	
	
	export class PersonView {
	    last: string;
	    first: string;
	    middle: string;
	    subject: string;
	    fromFile: boolean;
	    teacher: boolean;
	
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
	    }
	}
	
	
	
	
	
	
	
	export class View {
	    product: string;
	    spreads: SpreadView[];
	    settings: Settings;
	    folders: string[];
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
	    textMaxW: number;
	    backdropUsed: string[];
	    libPhotos: string[];
	    marksRev: number;
	    unfilled: string[];
	    unnamed: number;
	    cover?: CoverView;
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.product = source["product"];
	        this.spreads = this.convertValues(source["spreads"], SpreadView);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.folders = source["folders"];
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
	        this.textMaxW = source["textMaxW"];
	        this.backdropUsed = source["backdropUsed"];
	        this.libPhotos = source["libPhotos"];
	        this.marksRev = source["marksRev"];
	        this.unfilled = source["unfilled"];
	        this.unnamed = source["unnamed"];
	        this.cover = this.convertValues(source["cover"], CoverView);
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
	
	    static createFrom(source: any = {}) {
	        return new Prefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.showStart = source["showStart"];
	        this.first = source["first"];
	        this.custom = this.convertValues(source["custom"], CustomLevel);
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

