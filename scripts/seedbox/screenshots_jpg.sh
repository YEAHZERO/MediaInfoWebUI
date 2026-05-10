#!/bin/bash

# MediaInfoWebUI Screenshot Script (JPG)
# Generates screenshot thumbnails from video files at given timestamps
# Supports: external subtitles, embedded subtitles (text/PGS), HDR->SDR tonemapping, JPG output
#
# Usage: screenshots_jpg.sh [-nosub] <input_file> <output_dir> <timestamps...>

set -euo pipefail

# --- Configuration ---
FONT_DIR="${FONT_DIR:-/usr/share/fonts}"
EXTRACTED_FONTS_DIR=""
NOSUB=0
INPUT_FILE=""
OUTPUT_DIR=""
TIMESTAMPS=()
FONTEXTRACT_BIN=""

# --- Argument Parsing ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        -nosub)
            NOSUB=1
            shift
            ;;
        -*)
            echo "Unknown option: $1"
            exit 1
            ;;
        *)
            if [[ -z "$INPUT_FILE" ]]; then
                INPUT_FILE="$1"
            elif [[ -z "$OUTPUT_DIR" ]]; then
                OUTPUT_DIR="$1"
            else
                TIMESTAMPS+=("$1")
            fi
            shift
            ;;
    esac
done

if [[ -z "$INPUT_FILE" || -z "$OUTPUT_DIR" || ${#TIMESTAMPS[@]} -eq 0 ]]; then
    echo "Usage: $0 [-nosub] <input_file> <output_dir> <timestamps...>"
    exit 1
fi

mkdir -p "$OUTPUT_DIR"

# --- Utility Functions ---

cleanup_fonts() {
    if [[ -n "$EXTRACTED_FONTS_DIR" && -d "$EXTRACTED_FONTS_DIR" ]]; then
        rm -rf "$EXTRACTED_FONTS_DIR"
        EXTRACTED_FONTS_DIR=""
    fi
}
trap cleanup_fonts EXIT

find_binary() {
    local name="$1"
    local path="${2:-}"
    if [[ -n "$path" && -x "$(command -v "$path")" ]]; then
        echo "$path"
        return 0
    fi
    local found
    found="$(command -v "$name" 2>/dev/null)" || true
    if [[ -n "$found" && -x "$found" ]]; then
        echo "$found"
        return 0
    fi
    echo ""
    return 1
}

# --- Probe Functions ---

probe_video() {
    ffprobe -v error -select_streams v:0 \
        -show_entries "stream=width,height,codec_name:stream_side_data_list=side_data_type" \
        -of "default=noprint_wrappers=1" "$INPUT_FILE" 2>/dev/null
}

probe_color() {
    ffprobe -v error -select_streams v:0 \
        -show_entries "stream=color_space,color_primaries,color_transfer:stream_side_data_list=side_data_type,dv_profile" \
        -of "default=noprint_wrappers=1" "$INPUT_FILE" 2>/dev/null
}

# --- Color Space Detection ---

needs_tonemap() {
    local transfer primaries
    transfer=$(probe_color | grep "^color_transfer=" | cut -d= -f2 | tr -d ' ')
    primaries=$(probe_color | grep "^color_primaries=" | cut -d= -f2 | tr -d ' ')
    [[ -z "$transfer" && -z "$primaries" ]] && return 1
    if echo "$transfer" | grep -qiE "smpte2084|arib-std-b67|arib"; then
        return 0
    fi
    if echo "$primaries" | grep -qiE "bt2020"; then
        return 0
    fi
    return 1
}

# --- Subtitle Detection ---

detect_external_subtitles() {
    local dir ext base subtitle
    dir=$(dirname "$INPUT_FILE")
    base=$(basename "$INPUT_FILE")
    base="${base%.*}"

    for ext in "zh.ass" "chs.ass" "chi.ass" "sc.ass" "zh.srt" "chs.srt" "chi.srt" "sc.srt" "en.ass" "en.srt" "eng.ass" "eng.srt"; do
        subtitle="$dir/${base}.${ext}"
        if [[ -f "$subtitle" ]]; then
            echo "$subtitle"
            return 0
        fi
    done

    local best=""
    local best_size=0
    while IFS= read -r -d '' f; do
        local size
        size=$(stat -c%s "$f" 2>/dev/null || echo 0)
        if [[ "$size" -gt "$best_size" ]]; then
            best="$f"
            best_size="$size"
        fi
    done < <(find "$dir" -maxdepth 1 -type f \( -name "*.ass" -o -name "*.srt" \) -print0 2>/dev/null)
    echo "$best"
}

detect_internal_subtitles() {
    local sub_type="$1"
    local streams

    streams=$(ffprobe -v error -select_streams s \
        -show_entries "stream=index,codec_name:disposition=forced,default" \
        -of "csv=p=0" "$INPUT_FILE" 2>/dev/null)

    if [[ -z "$streams" ]]; then
        return 1
    fi

    local best_idx=""
    local best_priority=-1
    local fallback_idx=""
    local fallback_priority=-1

    while IFS= read -r line; do
        [[ -z "$line" ]] && continue
        IFS=',' read -r idx codec forced def <<< "$line"
        idx=$(echo "$idx" | tr -d ' ')
        codec=$(echo "$codec" | tr -d ' ')
        forced=$(echo "$forced" | tr -d ' ')
        def=$(echo "$def" | tr -d ' ')

        case "$sub_type" in
            text)
                case "$codec" in
                    subrip|srt|ass|ssa|mov_text) ;;
                    *) continue ;;
                esac
                ;;
            pgs)
                case "$codec" in
                    pgssub|hdmv_pgs_subtitle) ;;
                    *) continue ;;
                esac
                ;;
            *)
                continue
                ;;
        esac

        local is_chinese=0
        local meta_lang
        meta_lang=$(ffprobe -v error -select_streams "s:$idx" -show_entries stream_tags=language -of default=noprint_wrappers=1:nokey=1 "$INPUT_FILE" 2>/dev/null)
        case "$meta_lang" in
            chi|zho|zh|chs|cht|cn) is_chinese=1 ;;
        esac

        local priority=0
        if [[ "$forced" == "1" && "$is_chinese" == "1" ]]; then priority=5
        elif [[ "$forced" == "1" ]]; then priority=4
        elif [[ "$def" == "1" && "$is_chinese" == "1" ]]; then priority=3
        elif [[ "$def" == "1" ]]; then priority=2
        elif [[ "$is_chinese" == "1" ]]; then priority=1
        else priority=0
        fi

        if [[ "$priority" -gt "$best_priority" ]]; then
            fallback_priority="$best_priority"
            fallback_idx="$best_idx"
            best_priority="$priority"
            best_idx="$idx"
        elif [[ "$priority" -gt "$fallback_priority" ]]; then
            fallback_priority="$priority"
            fallback_idx="$idx"
        fi
    done <<< "$streams"

    echo "$best_idx"
}

detect_pgs_subtitles() {
    detect_internal_subtitles "pgs"
}

detect_text_subtitles() {
    detect_internal_subtitles "text"
}

# --- Font Extraction ---

extract_embedded_fonts() {
    local ext
    ext=$(echo "${INPUT_FILE##*.}" | tr '[:upper:]' '[:lower:]')
    if [[ "$ext" != "mkv" && "$ext" != "mka" ]]; then
        return 1
    fi

    FONTEXTRACT_BIN=$(find_binary "mkvextract" "")
    if [[ -z "$FONTEXTRACT_BIN" ]]; then
        return 1
    fi

    local font_attachments
    font_attachments=$("$FONTEXTRACT_BIN" attachments "$INPUT_FILE" 2>/dev/null) || true
    if [[ -z "$font_attachments" ]] || echo "$font_attachments" | grep -qi "no attachments"; then
        return 1
    fi

    EXTRACTED_FONTS_DIR=$(mktemp -d)
    "$FONTEXTRACT_BIN" attachments "$INPUT_FILE" --output-directory "$EXTRACTED_FONTS_DIR" >/dev/null 2>&1 || true

    local count
    count=$(find "$EXTRACTED_FONTS_DIR" -type f 2>/dev/null | wc -l)
    if [[ "$count" -eq 0 ]]; then
        rm -rf "$EXTRACTED_FONTS_DIR"
        EXTRACTED_FONTS_DIR=""
        return 1
    fi
    return 0
}

# --- Filter Builders ---

build_text_sub_filter() {
    local stream_idx="$1"
    local sub_path="$2"
    local filter

    filter="subtitles="

    if [[ -n "$sub_path" ]]; then
        local escaped
        escaped=$(echo "$sub_path" | sed "s/:/\\\\:/g; s/'/'\\\\\\\\''/g")
        filter+="'$escaped'"
    else
        filter+="'$INPUT_FILE'"
    fi

    if [[ -n "$stream_idx" ]]; then
        filter+=":si=$stream_idx"
    fi

    filter+=":fontsdir='$FONT_DIR'"

    if [[ -n "$EXTRACTED_FONTS_DIR" ]]; then
        filter+=":fontsdir='$EXTRACTED_FONTS_DIR'"
    fi

    filter+=":force_style='FontName=Noto Sans CJK SC,FontSize=18,Outline=1,Shadow=1'"
    echo "$filter"
}

build_overlay_vf() {
    local scale
    local filter

    scale=$(probe_video | grep "^width=" | cut -d= -f2 | tr -d ' ')
    if [[ -n "$scale" && "$scale" -gt 1920 ]]; then
        filter="scale=1920:-2,setsar=1:1"
    else
        filter="scale=1920:-2"
    fi

    if needs_tonemap; then
        filter+=",tonemap=tonemap=hable:desat=2"
    fi

    echo "$filter"
}

# --- Screenshot Capture ---

do_screenshot() {
    local timestamp="$1"
    local output="$2"
    local vf=""
    local has_text_subtitle=0
    local has_pgs_subtitle=0
    local text_stream_idx=""
    local pgs_stream_idx=""
    local external_sub=""

    if [[ "$NOSUB" -eq 0 ]]; then
        external_sub=$(detect_external_subtitles)
        text_stream_idx=$(detect_text_subtitles)
        pgs_stream_idx=$(detect_pgs_subtitles)

        if [[ -n "$text_stream_idx" || -n "$external_sub" ]]; then
            has_text_subtitle=1
        fi
        if [[ -n "$pgs_stream_idx" ]]; then
            has_pgs_subtitle=1
        fi
    fi

    local pgs_png=""
    if [[ "$has_pgs_subtitle" -eq 1 ]]; then
        pgs_png=$(extract_pgs_overlay "$timestamp" "$pgs_stream_idx")
    fi

    if [[ "$has_text_subtitle" -eq 1 ]]; then
        vf=$(build_text_sub_filter "$text_stream_idx" "$external_sub")
    elif [[ -n "$pgs_png" ]]; then
        local scale_filter
        scale_filter=$(build_overlay_vf)
        vf="$scale_filter"
    else
        vf=$(build_overlay_vf)
    fi

    local ffmpeg_args=(
        ffmpeg -y -ss "$timestamp" -i "$INPUT_FILE"
        -vframes 1 -an
    )

    if [[ -n "$vf" ]]; then
        ffmpeg_args+=(-vf "$vf")
    fi

    ffmpeg_args+=(
        -qscale:v 3
        "$output"
    )

    echo "  [ffmpeg] screenshot at $timestamp -> $(basename "$output")"
    "${ffmpeg_args[@]}" 2>/dev/null || {
        echo "  [fallback] retrying without tonemap..."
        local fallback_vf="$vf"
        if echo "$fallback_vf" | grep -q "tonemap"; then
            fallback_vf=$(echo "$fallback_vf" | sed 's/,tonemap=[^,]*//g')
            ffmpeg -y -ss "$timestamp" -i "$INPUT_FILE" \
                -vframes 1 -an \
                -vf "$fallback_vf" \
                -qscale:v 3 \
                "$output" 2>/dev/null || {
                echo "  [fallback] retrying with basic settings..."
                ffmpeg -y -ss "$timestamp" -i "$INPUT_FILE" \
                    -vframes 1 -an -qscale:v 3 \
                    "$output" 2>/dev/null
            }
        fi
    }

    if [[ -n "$pgs_png" && -f "$output" && -f "$pgs_png" ]]; then
        composite_pgs "$output" "$pgs_png"
        rm -f "$pgs_png"
    fi
}

extract_pgs_overlay() {
    local timestamp="$1"
    local stream_idx="$2"
    local pgs_output="$OUTPUT_DIR/pgs_overlay_${timestamp//:/_}.png"

    ffmpeg -y -ss "$timestamp" -i "$INPUT_FILE" \
        -map "0:s:$stream_idx" \
        -vframes 1 -an \
        -compression_level 3 \
        "$pgs_output" 2>/dev/null && [[ -f "$pgs_output" ]] && echo "$pgs_output" || echo ""
}

composite_pgs() {
    local screenshot="$1"
    local overlay="$2"

    local vw vh
    vw=$(probe_video | grep "^width=" | cut -d= -f2 | tr -d ' ')
    vh=$(probe_video | grep "^height=" | cut -d= -f2 | tr -d ' ')

    local ow oh
    ow=$(ffprobe -v error -select_streams v:0 -show_entries stream=width -of default=noprint_wrappers=1:nokey=1 "$overlay" 2>/dev/null | head -1)
    oh=$(ffprobe -v error -select_streams v:0 -show_entries stream=height -of default=noprint_wrappers=1:nokey=1 "$overlay" 2>/dev/null | head -1)

    if [[ -n "$vw" && -n "$vh" && -n "$ow" && -n "$oh" ]]; then
        if [[ "$ow" -eq "$vw" && "$oh" -le "$vh" ]]; then
            local y_offset=$((vh - oh - 10))
            ffmpeg -y -i "$screenshot" -i "$overlay" \
                -filter_complex "[0:v][1:v]overlay=0:$y_offset" \
                -qscale:v 3 \
                "$screenshot.tmp.jpg" 2>/dev/null && mv "$screenshot.tmp.jpg" "$screenshot"
        else
            ffmpeg -y -i "$screenshot" -i "$overlay" \
                -filter_complex "[1:v]scale=$vw:-2[sub];[0:v][sub]overlay=(W-w)/2:(H-h-10)" \
                -qscale:v 3 \
                "$screenshot.tmp.jpg" 2>/dev/null && mv "$screenshot.tmp.jpg" "$screenshot"
        fi
    else
        ffmpeg -y -i "$screenshot" -i "$overlay" \
            -filter_complex "[0:v][1:v]overlay=(W-w)/2:(H-h-10)" \
            -qscale:v 3 \
            "$screenshot.tmp.jpg" 2>/dev/null && mv "$screenshot.tmp.jpg" "$screenshot"
    fi
}

# --- Main ---

echo "=== Screenshot Generation (JPG) ==="
echo "Input: $INPUT_FILE"
echo "Output: $OUTPUT_DIR"
echo "Timestamps: ${TIMESTAMPS[*]}"
[[ "$NOSUB" -eq 1 ]] && echo "Subtitles: disabled" || echo "Subtitles: auto"

if [[ "$NOSUB" -eq 0 ]]; then
    extract_embedded_fonts && echo "Fonts: extracted to $EXTRACTED_FONTS_DIR" || echo "Fonts: no embedded fonts found"
fi

for ts in "${TIMESTAMPS[@]}"; do
    outname="screenshot_$(echo "$ts" | tr ':' '_').jpg"
    do_screenshot "$ts" "$OUTPUT_DIR/$outname"
done

echo "=== Done ==="