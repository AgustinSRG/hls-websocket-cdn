// MP4 tools, from HLs.js
// https://github.com/video-dev/hls.js/

"use strict";


export interface InitDataTrack {
    timescale: number;
    id: number;
    codec: string;
}

type HdlrType = "audio" | "video";

export interface InitData extends Array<any> {
    [index: number]:
    | {
        timescale: number;
        type: HdlrType;
        default?: {
            duration: number;
            flags: number;
        };
    }
    | undefined;
    audio?: InitDataTrack;
    video?: InitDataTrack;
    caption?: InitDataTrack;
}

export function readSint32(buffer: Uint8Array, offset: number): number {
    return (
        (buffer[offset] << 24) |
        (buffer[offset + 1] << 16) |
        (buffer[offset + 2] << 8) |
        buffer[offset + 3]
    );
}

export function readUint32(buffer: Uint8Array, offset: number): number {
    const val = readSint32(buffer, offset);
    return val < 0 ? 4294967296 + val : val;
}

export function bin2str(data: Uint8Array): string {
    return String.fromCharCode.apply(null, data);
}

// Find the data for a box specified by its path
export function findBox(data: Uint8Array, path: string[]): Uint8Array[] {
    const results = [] as Uint8Array[];
    if (!path.length) {
        // short-circuit the search for empty paths
        return results;
    }
    const end = data.byteLength;

    for (let i = 0; i < end;) {
        const size = readUint32(data, i);
        const type = bin2str(data.subarray(i + 4, i + 8));
        const endbox = size > 1 ? i + size : end;
        if (type === path[0]) {
            if (path.length === 1) {
                // this is the end of the path and we've found the box we were
                // looking for
                results.push(data.subarray(i + 8, endbox));
            } else {
                // recursively search for the next box along the path
                const subresults = findBox(data.subarray(i + 8, endbox), path.slice(1));
                if (subresults.length) {
                    [].push.apply(results, subresults);
                }
            }
        }
        i = endbox;
    }

    // we've finished searching all of data
    return results;
}

export function parseInitSegment(initSegment: Uint8Array): InitData {
    const result: InitData = [];
    const traks = findBox(initSegment, ['moov', 'trak']);
    for (let i = 0; i < traks.length; i++) {
        const trak = traks[i];
        const tkhd = findBox(trak, ['tkhd'])[0];
        if (tkhd) {
            let version = tkhd[0];
            const trackId = readUint32(tkhd, version === 0 ? 12 : 20);
            const mdhd = findBox(trak, ['mdia', 'mdhd'])[0];
            if (mdhd) {
                version = mdhd[0];
                const timescale = readUint32(mdhd, version === 0 ? 12 : 20);
                const hdlr = findBox(trak, ['mdia', 'hdlr'])[0];
                if (hdlr) {
                    const hdlrType = bin2str(hdlr.subarray(8, 12));
                    const type = {
                        soun: "audio",
                        vide: "video",
                    }[hdlrType] as HdlrType;
                    if (type) {
                        // Parse codec details
                        const stsd = findBox(trak, ['mdia', 'minf', 'stbl', 'stsd'])[0];
                        const stsdData = parseStsd(stsd);
                        result[trackId] = { timescale, type };
                        result[type] = { timescale, id: trackId, ...stsdData };
                    }
                }
            }
        }
    }

    const trex = findBox(initSegment, ['moov', 'mvex', 'trex']);
    trex.forEach((trex) => {
        const trackId = readUint32(trex, 4);
        const track = result[trackId];
        if (track) {
            track.default = {
                duration: readUint32(trex, 12),
                flags: readUint32(trex, 20),
            };
        }
    });

    return result;
}

function addLeadingZero(num: number): string {
    return (num < 10 ? '0' : '') + num;
}

function toHex(x: number): string {
    return ('0' + x.toString(16).toUpperCase()).slice(-2);
}

function skipBERInteger(bytes: Uint8Array, i: number): number {
    const limit = i + 5;
    while (bytes[i++] & 0x80 && i < limit) {
        /* do nothing */
    }
    return i;
}

function parseStsd(stsd: Uint8Array): { codec: string; encrypted: boolean } {
    const sampleEntries = stsd.subarray(8);
    const sampleEntriesEnd = sampleEntries.subarray(8 + 78);
    const fourCC = bin2str(sampleEntries.subarray(4, 8));
    let codec = fourCC;
    const encrypted = fourCC === 'enca' || fourCC === 'encv';
    if (encrypted) {
        const encBox = findBox(sampleEntries, [fourCC])[0];
        const encBoxChildren = encBox.subarray(fourCC === 'enca' ? 28 : 78);
        const sinfs = findBox(encBoxChildren, ['sinf']);
        sinfs.forEach((sinf) => {
            const schm = findBox(sinf, ['schm'])[0];
            if (schm) {
                const scheme = bin2str(schm.subarray(4, 8));
                if (scheme === 'cbcs' || scheme === 'cenc') {
                    const frma = findBox(sinf, ['frma'])[0];
                    if (frma) {
                        // for encrypted content codec fourCC will be in frma
                        codec = bin2str(frma);
                    }
                }
            }
        });
    }
    switch (codec) {
        case 'avc1':
        case 'avc2':
        case 'avc3':
        case 'avc4': {
            // extract profile + compatibility + level out of avcC box
            const avcCBox = findBox(sampleEntriesEnd, ['avcC'])[0];
            codec += '.' + toHex(avcCBox[1]) + toHex(avcCBox[2]) + toHex(avcCBox[3]);
            break;
        }
        case 'mp4a': {
            const codecBox = findBox(sampleEntries, [fourCC])[0];
            const esdsBox = findBox(codecBox.subarray(28), ['esds'])[0];
            if (esdsBox && esdsBox.length > 7) {
                let i = 4;
                // ES Descriptor tag
                if (esdsBox[i++] !== 0x03) {
                    break;
                }
                i = skipBERInteger(esdsBox, i);
                i += 2; // skip es_id;
                const flags = esdsBox[i++];
                if (flags & 0x80) {
                    i += 2; // skip dependency es_id
                }
                if (flags & 0x40) {
                    i += esdsBox[i++]; // skip URL
                }
                // Decoder config descriptor
                if (esdsBox[i++] !== 0x04) {
                    break;
                }
                i = skipBERInteger(esdsBox, i);
                const objectType = esdsBox[i++];
                if (objectType === 0x40) {
                    codec += '.' + toHex(objectType);
                } else {
                    break;
                }
                i += 12;
                // Decoder specific info
                if (esdsBox[i++] !== 0x05) {
                    break;
                }
                i = skipBERInteger(esdsBox, i);
                const firstByte = esdsBox[i++];
                let audioObjectType = (firstByte & 0xf8) >> 3;
                if (audioObjectType === 31) {
                    audioObjectType +=
                        1 + ((firstByte & 0x7) << 3) + ((esdsBox[i] & 0xe0) >> 5);
                }
                codec += '.' + audioObjectType;
            }
            break;
        }
        case 'hvc1':
        case 'hev1': {
            const hvcCBox = findBox(sampleEntriesEnd, ['hvcC'])[0];
            const profileByte = hvcCBox[1];
            const profileSpace = ['', 'A', 'B', 'C'][profileByte >> 6];
            const generalProfileIdc = profileByte & 0x1f;
            const profileCompat = readUint32(hvcCBox, 2);
            const tierFlag = (profileByte & 0x20) >> 5 ? 'H' : 'L';
            const levelIDC = hvcCBox[12];
            const constraintIndicator = hvcCBox.subarray(6, 12);
            codec += '.' + profileSpace + generalProfileIdc;
            codec += '.' + profileCompat.toString(16).toUpperCase();
            codec += '.' + tierFlag + levelIDC;
            let constraintString = '';
            for (let i = constraintIndicator.length; i--;) {
                const byte = constraintIndicator[i];
                if (byte || constraintString) {
                    const encodedByte = byte.toString(16).toUpperCase();
                    constraintString = '.' + encodedByte + constraintString;
                }
            }
            codec += constraintString;
            break;
        }
        case 'dvh1':
        case 'dvhe': {
            const dvcCBox = findBox(sampleEntriesEnd, ['dvcC'])[0];
            const profile = (dvcCBox[2] >> 1) & 0x7f;
            const level = ((dvcCBox[2] << 5) & 0x20) | ((dvcCBox[3] >> 3) & 0x1f);
            codec += '.' + addLeadingZero(profile) + '.' + addLeadingZero(level);
            break;
        }
        case 'vp09': {
            const vpcCBox = findBox(sampleEntriesEnd, ['vpcC'])[0];
            const profile = vpcCBox[4];
            const level = vpcCBox[5];
            const bitDepth = (vpcCBox[6] >> 4) & 0x0f;
            codec +=
                '.' +
                addLeadingZero(profile) +
                '.' +
                addLeadingZero(level) +
                '.' +
                addLeadingZero(bitDepth);
            break;
        }
        case 'av01': {
            const av1CBox = findBox(sampleEntriesEnd, ['av1C'])[0];
            const profile = av1CBox[1] >>> 5;
            const level = av1CBox[1] & 0x1f;
            const tierFlag = av1CBox[2] >>> 7 ? 'H' : 'M';
            const highBitDepth = (av1CBox[2] & 0x40) >> 6;
            const twelveBit = (av1CBox[2] & 0x20) >> 5;
            const bitDepth =
                profile === 2 && highBitDepth
                    ? twelveBit
                        ? 12
                        : 10
                    : highBitDepth
                        ? 10
                        : 8;
            const monochrome = (av1CBox[2] & 0x10) >> 4;
            const chromaSubsamplingX = (av1CBox[2] & 0x08) >> 3;
            const chromaSubsamplingY = (av1CBox[2] & 0x04) >> 2;
            const chromaSamplePosition = av1CBox[2] & 0x03;
            // TODO: parse color_description_present_flag
            // default it to BT.709/limited range for now
            // more info https://aomediacodec.github.io/av1-isobmff/#av1codecconfigurationbox-syntax
            const colorPrimaries = 1;
            const transferCharacteristics = 1;
            const matrixCoefficients = 1;
            const videoFullRangeFlag = 0;
            codec +=
                '.' +
                profile +
                '.' +
                addLeadingZero(level) +
                tierFlag +
                '.' +
                addLeadingZero(bitDepth) +
                '.' +
                monochrome +
                '.' +
                chromaSubsamplingX +
                chromaSubsamplingY +
                chromaSamplePosition +
                '.' +
                addLeadingZero(colorPrimaries) +
                '.' +
                addLeadingZero(transferCharacteristics) +
                '.' +
                addLeadingZero(matrixCoefficients) +
                '.' +
                videoFullRangeFlag;
            break;
        }
        case 'ac-3':
        case 'ec-3':
        case 'alac':
        case 'fLaC':
        case 'Opus':
        default:
            break;
    }
    return { codec, encrypted };
}

