export namespace types {
	
	export class EncoderCompensationConfig {
	    enabled: boolean;
	    tolerance: number;
	    maxCycles: number;
	    settleMs: number;
	    minStep: number;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new EncoderCompensationConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.tolerance = source["tolerance"];
	        this.maxCycles = source["maxCycles"];
	        this.settleMs = source["settleMs"];
	        this.minStep = source["minStep"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class AxisConfig {
	    name: string;
	    enabled: boolean;
	    kind: string;
	    inverted: boolean;
	    stepAngleDeg: number;
	    microSteps: number;
	    lead: number;
	    gearRatio: number;
	    maxSpeed: number;
	    encoderScale: number;
	    encoderCompensation: EncoderCompensationConfig;
	
	    static createFrom(source: any = {}) {
	        return new AxisConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.kind = source["kind"];
	        this.inverted = source["inverted"];
	        this.stepAngleDeg = source["stepAngleDeg"];
	        this.microSteps = source["microSteps"];
	        this.lead = source["lead"];
	        this.gearRatio = source["gearRatio"];
	        this.maxSpeed = source["maxSpeed"];
	        this.encoderScale = source["encoderScale"];
	        this.encoderCompensation = this.convertValues(source["encoderCompensation"], EncoderCompensationConfig);
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
	export class AxisStatus {
	    name: string;
	    position: number;
	    moving: boolean;
	    homed: boolean;
	    posLimit: boolean;
	    negLimit: boolean;
	    compensating: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AxisStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.position = source["position"];
	        this.moving = source["moving"];
	        this.homed = source["homed"];
	        this.posLimit = source["posLimit"];
	        this.negLimit = source["negLimit"];
	        this.compensating = source["compensating"];
	    }
	}
	export class SphereTankGateConfig {
	    enabled: boolean;
	    channelIndex: number;
	    thresholdRate: number;
	    stableTimeMs: number;
	
	    static createFrom(source: any = {}) {
	        return new SphereTankGateConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.channelIndex = source["channelIndex"];
	        this.thresholdRate = source["thresholdRate"];
	        this.stableTimeMs = source["stableTimeMs"];
	    }
	}
	export class CalibrationPoint {
	    id: string;
	    alpha: number;
	    beta: number;
	
	    static createFrom(source: any = {}) {
	        return new CalibrationPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.alpha = source["alpha"];
	        this.beta = source["beta"];
	    }
	}
	export class ProbeChannelConfig {
	    name: string;
	    role: string;
	    channel: number;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProbeChannelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.role = source["role"];
	        this.channel = source["channel"];
	        this.enabled = source["enabled"];
	    }
	}
	export class CalibrationConfig {
	    type: string;
	    deviceId: string;
	    controllerId: string;
	    probeChannels: ProbeChannelConfig[];
	    alphaAxis: string;
	    betaAxis: string;
	    points: CalibrationPoint[];
	    dwellTimeMs: number;
	    samplesPerPoint: number;
	    sphereTankGate: SphereTankGateConfig;
	
	    static createFrom(source: any = {}) {
	        return new CalibrationConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.deviceId = source["deviceId"];
	        this.controllerId = source["controllerId"];
	        this.probeChannels = this.convertValues(source["probeChannels"], ProbeChannelConfig);
	        this.alphaAxis = source["alphaAxis"];
	        this.betaAxis = source["betaAxis"];
	        this.points = this.convertValues(source["points"], CalibrationPoint);
	        this.dwellTimeMs = source["dwellTimeMs"];
	        this.samplesPerPoint = source["samplesPerPoint"];
	        this.sphereTankGate = this.convertValues(source["sphereTankGate"], SphereTankGateConfig);
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
	export class FiveHoleCoefficients {
	    Kalpha: number;
	    Kbeta: number;
	    CPT: number;
	    CPS: number;
	
	    static createFrom(source: any = {}) {
	        return new FiveHoleCoefficients(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Kalpha = source["Kalpha"];
	        this.Kbeta = source["Kbeta"];
	        this.CPT = source["CPT"];
	        this.CPS = source["CPS"];
	    }
	}
	export class FiveHoleRawData {
	    p1: number;
	    p2: number;
	    p3: number;
	    p4: number;
	    p5: number;
	    pAtm: number;
	    tAtm: number;
	    pTotal?: number;
	
	    static createFrom(source: any = {}) {
	        return new FiveHoleRawData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.p1 = source["p1"];
	        this.p2 = source["p2"];
	        this.p3 = source["p3"];
	        this.p4 = source["p4"];
	        this.p5 = source["p5"];
	        this.pAtm = source["pAtm"];
	        this.tAtm = source["tAtm"];
	        this.pTotal = source["pTotal"];
	    }
	}
	export class CalibrationDataPoint {
	    pointId: string;
	    alpha: number;
	    beta: number;
	    rawData: FiveHoleRawData;
	    coefficients: FiveHoleCoefficients;
	    sampleCount: number;
	    stdDev: number;
	
	    static createFrom(source: any = {}) {
	        return new CalibrationDataPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pointId = source["pointId"];
	        this.alpha = source["alpha"];
	        this.beta = source["beta"];
	        this.rawData = this.convertValues(source["rawData"], FiveHoleRawData);
	        this.coefficients = this.convertValues(source["coefficients"], FiveHoleCoefficients);
	        this.sampleCount = source["sampleCount"];
	        this.stdDev = source["stdDev"];
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
	
	export class CalibrationTaskStatus {
	    taskId: string;
	    status: string;
	    totalPoints: number;
	    completedPoints: number;
	    progress: number;
	    currentPoint?: CalibrationPoint;
	    dataPoints: CalibrationDataPoint[];
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new CalibrationTaskStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.status = source["status"];
	        this.totalPoints = source["totalPoints"];
	        this.completedPoints = source["completedPoints"];
	        this.progress = source["progress"];
	        this.currentPoint = this.convertValues(source["currentPoint"], CalibrationPoint);
	        this.dataPoints = this.convertValues(source["dataPoints"], CalibrationDataPoint);
	        this.lastError = source["lastError"];
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
	export class ChannelConfig {
	    index: number;
	    name: string;
	    enabled: boolean;
	    unit: string;
	    precision: number;
	    rangeMin: number;
	    rangeMax: number;
	
	    static createFrom(source: any = {}) {
	        return new ChannelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.unit = source["unit"];
	        this.precision = source["precision"];
	        this.rangeMin = source["rangeMin"];
	        this.rangeMax = source["rangeMax"];
	    }
	}
	export class DataPayload {
	    deviceId: string;
	    timestamp: number;
	    channels: number[];
	    channelIndices: number[];
	
	    static createFrom(source: any = {}) {
	        return new DataPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.timestamp = source["timestamp"];
	        this.channels = source["channels"];
	        this.channelIndices = source["channelIndices"];
	    }
	}
	export class DeviceProfile {
	    id: string;
	    name: string;
	    type: string;
	    host: string;
	    port: number;
	    streamId: number;
	    periodMs: number;
	    autoConnect: boolean;
	    channels: ChannelConfig[];
	
	    static createFrom(source: any = {}) {
	        return new DeviceProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.streamId = source["streamId"];
	        this.periodMs = source["periodMs"];
	        this.autoConnect = source["autoConnect"];
	        this.channels = this.convertValues(source["channels"], ChannelConfig);
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
	export class DeviceStatus {
	    id: string;
	    name: string;
	    type: string;
	    status: string;
	    acquiring: boolean;
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.acquiring = source["acquiring"];
	        this.lastError = source["lastError"];
	    }
	}
	export class DiscoveredDevice {
	    ip: string;
	    mac: string;
	    sn: string;
	    firmware: string;
	    port: number;
	    mask: string;
	    gateway: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.mac = source["mac"];
	        this.sn = source["sn"];
	        this.firmware = source["firmware"];
	        this.port = source["port"];
	        this.mask = source["mask"];
	        this.gateway = source["gateway"];
	    }
	}
	
	
	
	export class LimitStatus {
	    posLimit: boolean;
	    negLimit: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LimitStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.posLimit = source["posLimit"];
	        this.negLimit = source["negLimit"];
	    }
	}
	export class StepSegment {
	    start: number;
	    end: number;
	    step: number;
	
	    static createFrom(source: any = {}) {
	        return new StepSegment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.step = source["step"];
	    }
	}
	export class LineLayout {
	    startX: number;
	    startY: number;
	    endX: number;
	    endY: number;
	    xSteps: StepSegment[];
	    ySteps: StepSegment[];
	
	    static createFrom(source: any = {}) {
	        return new LineLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startX = source["startX"];
	        this.startY = source["startY"];
	        this.endX = source["endX"];
	        this.endY = source["endY"];
	        this.xSteps = this.convertValues(source["xSteps"], StepSegment);
	        this.ySteps = this.convertValues(source["ySteps"], StepSegment);
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
	export class MotionAxisMapping {
	    axis: string;
	
	    static createFrom(source: any = {}) {
	        return new MotionAxisMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.axis = source["axis"];
	    }
	}
	export class MotionControllerProfile {
	    id: string;
	    name: string;
	    type: string;
	    address: string;
	    port: number;
	    timeoutMs: number;
	    axes: AxisConfig[];
	
	    static createFrom(source: any = {}) {
	        return new MotionControllerProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.timeoutMs = source["timeoutMs"];
	        this.axes = this.convertValues(source["axes"], AxisConfig);
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
	export class MotionControllerStatus {
	    id: string;
	    name: string;
	    type: string;
	    status: string;
	    axes: AxisStatus[];
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new MotionControllerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.axes = this.convertValues(source["axes"], AxisStatus);
	        this.lastError = source["lastError"];
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
	
	export class RectangleLayout {
	    xMin: number;
	    xMax: number;
	    yMin: number;
	    yMax: number;
	    xSteps: StepSegment[];
	    ySteps: StepSegment[];
	
	    static createFrom(source: any = {}) {
	        return new RectangleLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.xMin = source["xMin"];
	        this.xMax = source["xMax"];
	        this.yMin = source["yMin"];
	        this.yMax = source["yMax"];
	        this.xSteps = this.convertValues(source["xSteps"], StepSegment);
	        this.ySteps = this.convertValues(source["ySteps"], StepSegment);
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
	
	
	export class ThreeHoleCalibFileInfo {
	    filePath: string;
	    fileName: string;
	    cMa: number;
	
	    static createFrom(source: any = {}) {
	        return new ThreeHoleCalibFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.fileName = source["fileName"];
	        this.cMa = source["cMa"];
	    }
	}
	export class ThreeHoleInterpolationResult {
	    ptProbe: number;
	    psProbe: number;
	    machProbe: number;
	    alphaProbe: number;
	    iterationCount: number;
	    converged: boolean;
	    valid: boolean;
	    errorMsg?: string;
	
	    static createFrom(source: any = {}) {
	        return new ThreeHoleInterpolationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ptProbe = source["ptProbe"];
	        this.psProbe = source["psProbe"];
	        this.machProbe = source["machProbe"];
	        this.alphaProbe = source["alphaProbe"];
	        this.iterationCount = source["iterationCount"];
	        this.converged = source["converged"];
	        this.valid = source["valid"];
	        this.errorMsg = source["errorMsg"];
	    }
	}
	export class ThreeHoleProbeChannelConfig {
	    name: string;
	    role: string;
	    channel: number;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ThreeHoleProbeChannelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.role = source["role"];
	        this.channel = source["channel"];
	        this.enabled = source["enabled"];
	    }
	}
	export class ThreeHoleRawData {
	    p1: number;
	    p2: number;
	    p3: number;
	    pAtm: number;
	    tAtm: number;
	
	    static createFrom(source: any = {}) {
	        return new ThreeHoleRawData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.p1 = source["p1"];
	        this.p2 = source["p2"];
	        this.p3 = source["p3"];
	        this.pAtm = source["pAtm"];
	        this.tAtm = source["tAtm"];
	    }
	}
	export class TraversalPoint {
	    id: string;
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new TraversalPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class TraversalLayout {
	    pattern: string;
	    line?: LineLayout;
	    rectangle?: RectangleLayout;
	    customPoints?: TraversalPoint[];
	
	    static createFrom(source: any = {}) {
	        return new TraversalLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pattern = source["pattern"];
	        this.line = this.convertValues(source["line"], LineLayout);
	        this.rectangle = this.convertValues(source["rectangle"], RectangleLayout);
	        this.customPoints = this.convertValues(source["customPoints"], TraversalPoint);
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
	export class ThreeHoleTraversalConfig {
	    name: string;
	    deviceId: string;
	    motionControllerId: string;
	    layout: TraversalLayout;
	    probeChannels: ThreeHoleProbeChannelConfig[];
	    motionAlpha: MotionAxisMapping;
	    motionBeta: MotionAxisMapping;
	    calibFiles: ThreeHoleCalibFileInfo[];
	    dwellTimeMs: number;
	    samplesPerPoint: number;
	    sampleIntervalMs: number;
	    motionTimeoutMs: number;
	    savePath: string;
	    saveFileName: string;
	
	    static createFrom(source: any = {}) {
	        return new ThreeHoleTraversalConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.deviceId = source["deviceId"];
	        this.motionControllerId = source["motionControllerId"];
	        this.layout = this.convertValues(source["layout"], TraversalLayout);
	        this.probeChannels = this.convertValues(source["probeChannels"], ThreeHoleProbeChannelConfig);
	        this.motionAlpha = this.convertValues(source["motionAlpha"], MotionAxisMapping);
	        this.motionBeta = this.convertValues(source["motionBeta"], MotionAxisMapping);
	        this.calibFiles = this.convertValues(source["calibFiles"], ThreeHoleCalibFileInfo);
	        this.dwellTimeMs = source["dwellTimeMs"];
	        this.samplesPerPoint = source["samplesPerPoint"];
	        this.sampleIntervalMs = source["sampleIntervalMs"];
	        this.motionTimeoutMs = source["motionTimeoutMs"];
	        this.savePath = source["savePath"];
	        this.saveFileName = source["saveFileName"];
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
	export class ThreeHoleTraversalDataPoint {
	    pointId: string;
	    x: number;
	    y: number;
	    rawData: ThreeHoleRawData;
	    interpResult: ThreeHoleInterpolationResult;
	    sampleCount: number;
	    timestamp: number;
	
	    static createFrom(source: any = {}) {
	        return new ThreeHoleTraversalDataPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pointId = source["pointId"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.rawData = this.convertValues(source["rawData"], ThreeHoleRawData);
	        this.interpResult = this.convertValues(source["interpResult"], ThreeHoleInterpolationResult);
	        this.sampleCount = source["sampleCount"];
	        this.timestamp = source["timestamp"];
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
	export class ThreeHoleTraversalTaskStatus {
	    taskId: string;
	    status: string;
	    totalPoints: number;
	    completedPoints: number;
	    progress: number;
	    currentPoint?: TraversalPoint;
	    dataPoints: ThreeHoleTraversalDataPoint[];
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new ThreeHoleTraversalTaskStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.status = source["status"];
	        this.totalPoints = source["totalPoints"];
	        this.completedPoints = source["completedPoints"];
	        this.progress = source["progress"];
	        this.currentPoint = this.convertValues(source["currentPoint"], TraversalPoint);
	        this.dataPoints = this.convertValues(source["dataPoints"], ThreeHoleTraversalDataPoint);
	        this.lastError = source["lastError"];
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

