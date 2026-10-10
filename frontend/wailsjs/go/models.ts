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
	export class ExportOptions {
	    range: string;
	    quality: number;
	    dpi: number;
	    pages: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.range = source["range"];
	        this.quality = source["quality"];
	        this.dpi = source["dpi"];
	        this.pages = source["pages"];
	    }
	}
	export class GridResult {
	    view: model.View;
	    left: number;
	
	    static createFrom(source: any = {}) {
	        return new GridResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = this.convertValues(source["view"], model.View);
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
	    z: number;
	    over: boolean;
	    overflow: number;
	
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
	        this.z = source["z"];
	        this.over = source["over"];
	        this.overflow = source["overflow"];
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
	
	    static createFrom(source: any = {}) {
	        return new Choice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.rects = this.convertValues(source["rects"], layout.Rect);
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
	
	    static createFrom(source: any = {}) {
	        return new ChoiceRows(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inOrder = this.convertValues(source["inOrder"], Choice);
	        this.rearranged = this.convertValues(source["rearranged"], Choice);
	        this.block = source["block"];
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
	
	    static createFrom(source: any = {}) {
	        return new GridPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.frozen = source["frozen"];
	        this.left = source["left"];
	        this.texts = source["texts"];
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
	    }
	}
	export class Settings {
	    pageW: number;
	    pageH: number;
	    margin: number;
	    gap: number;
	    layflat: boolean;
	    dpi: number;
	    bg: string;
	
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
	        this.dpi = source["dpi"];
	        this.bg = source["bg"];
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
	export class SpreadView {
	    id: string;
	    photos: string[];
	    rects: layout.Rect[];
	    layout: string;
	    variant: number;
	    variants: number;
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
	
	    static createFrom(source: any = {}) {
	        return new SpreadView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.photos = source["photos"];
	        this.rects = this.convertValues(source["rects"], layout.Rect);
	        this.layout = source["layout"];
	        this.variant = source["variant"];
	        this.variants = source["variants"];
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
	    path: string;
	    dirty: boolean;
	    canUndo: boolean;
	    canRedo: boolean;
	    rev: number;
	    maxSpreadPhotos: number;
	    safe: layout.Rect[];
	    minDPI: number;
	    lowDPI: number;
	    emptyPlaces: number;
	    textMaxW: number;
	    backdropUsed: string[];
	    marksRev: number;
	
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
	        this.minDPI = source["minDPI"];
	        this.lowDPI = source["lowDPI"];
	        this.emptyPlaces = source["emptyPlaces"];
	        this.textMaxW = source["textMaxW"];
	        this.backdropUsed = source["backdropUsed"];
	        this.marksRev = source["marksRev"];
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
	    faces: Face[];
	
	    static createFrom(source: any = {}) {
	        return new Family(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.css = source["css"];
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

