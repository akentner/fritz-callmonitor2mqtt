# Fritz!Box Extension Mapping

This document explains how Fritz!Box extension numbers are mapped between the GUI configuration and callmonitor events based on systematic internal number ranges.

## Problem

The Fritz!Box web GUI shows different extension numbers than what appears in the callmonitor events:

- **GUI shows:** Extension 621
- **Callmonitor receives:** Extension 21
- **GUI shows:** Extension 615  
- **Callmonitor receives:** Extension 15
- **GUI shows:** AB 22 (Anrufbeantworter)
- **Callmonitor receives:** Extension 40

## Systematic Mapping Rules

Fritz!Box uses systematic internal **6xx number ranges to determine extension types and event numbers:

### Internal **6xx Number Ranges

Fritz!Box assigns extension types based on internal number ranges:

- **\*\*600 to \*\*609**: VOICEBOX → Events 40-49
- **\*\*610 to \*\*619**: DECT → Events 10-19  
- **\*\*620 to \*\*629**: VOIP → Events 20-29
- **Other ranges**: UNKNOWN type

### 6xx GUI Extensions
**Rule:** Remove the leading "6" from GUI numbers, type determined by range
- GUI `621` → Event `21` → Type VOIP (620-629 range)
- GUI `615` → Event `15` → Type DECT (610-619 range)
- GUI `610` → Event `10` → Type DECT (610-619 range)
- GUI `622` → Event `22` → Type VOIP (620-629 range)

### Anrufbeantworter (Voicebox) Extensions
**Rule:** Only internal **600-609 numbers are VOICEBOX extensions
- Internal `**600` → Event `40` → Type VOICEBOX
- Internal `**605` → Event `45` → Type VOICEBOX
- Names like "AB 22" or "AB 24" are just labels and don't follow systematic mapping
- GUI `AB 23` (Anrufbeantworter) → Internal `**623` → Event `42` *(hypothesis)*

**Pattern:** Anrufbeantworter (AB) uses internal **6xx numbers that get truncated to xx in events

### Direct Extensions
**Rule:** No mapping needed
- Extension `21` → Event `21`
- Extension `1` → Event `1`
- Extension `42` → Event `42`

## Configuration

When configuring extensions, use the **GUI numbers** from the Fritz!Box web interface:

```env
# Use GUI number 621 (will map to event 21 automatically)
FRITZ_CALLMONITOR_PBX_EXTENSION_0_NUMBER=621
FRITZ_CALLMONITOR_PBX_EXTENSION_0_NAME=Haupttelefon DECT
FRITZ_CALLMONITOR_PBX_EXTENSION_0_TYPE=DECT

# VoIP Phone (GUI: 621 -> Event: 21, Type: VOIP - 620-629 range)
FRITZ_CALLMONITOR_PBX_EXTENSION_0_NUMBER=621
FRITZ_CALLMONITOR_PBX_EXTENSION_0_NAME=Büro VoIP
FRITZ_CALLMONITOR_PBX_EXTENSION_0_TYPE=VOIP

# DECT Phone (GUI: 615 -> Event: 15, Type: DECT - 610-619 range)
FRITZ_CALLMONITOR_PBX_EXTENSION_1_NUMBER=615
FRITZ_CALLMONITOR_PBX_EXTENSION_1_NAME=Wohnzimmer DECT
FRITZ_CALLMONITOR_PBX_EXTENSION_1_TYPE=DECT

# Anrufbeantworter (Internal: **600 -> Event: 40, Type: VOICEBOX)
FRITZ_CALLMONITOR_PBX_EXTENSION_2_NUMBER=**600  
FRITZ_CALLMONITOR_PBX_EXTENSION_2_NAME=Anrufbeantworter Wohnzimmer
FRITZ_CALLMONITOR_PBX_EXTENSION_2_TYPE=VOICEBOX

# Internal VOICEBOX (Internal: **605 -> Event: 45, Type: VOICEBOX)
FRITZ_CALLMONITOR_PBX_EXTENSION_3_NUMBER=**605
FRITZ_CALLMONITOR_PBX_EXTENSION_3_NAME=Voicebox System
FRITZ_CALLMONITOR_PBX_EXTENSION_3_TYPE=VOICEBOX

# Unknown extension (Other ranges -> Type: UNKNOWN)
FRITZ_CALLMONITOR_PBX_EXTENSION_4_NUMBER=42
FRITZ_CALLMONITOR_PBX_EXTENSION_4_NAME=Legacy Extension
FRITZ_CALLMONITOR_PBX_EXTENSION_4_TYPE=UNKNOWN
```

## Lookup Process

The extension lookup works in two phases:

1. **Direct Lookup**: Try to find extension by the exact event number
2. **Mapped Lookup**: If not found, try to find by mapped GUI number

This allows both direct extensions and GUI-configured extensions to work seamlessly.

## Example

Live callmonitor event:
```
13.09.25 18:04:07;CALL;1;15;990135;3698237;SIP2;
```

- Event extension: `15`
- System looks for GUI extension: `615` 
- Returns extension info for GUI `615` with name and type

Another example with VOICEBOX:
```  
13.09.25 18:08:43;CONNECT;0;40;01783278576;
```

- Event extension: `40` (from internal **600)
- System looks for configured extension: `**600`
- Returns extension info for `**600` with name "Anrufbeantworter" and type "VOICEBOX"

## Verification

To verify your extension mappings:

1. Check your Fritz!Box web GUI for extension numbers
2. Monitor callmonitor events to see event extension numbers
3. Configure using GUI numbers - the system handles mapping automatically

## Unknown Patterns

Some patterns may need verification with more data:
- AB 23 (Anrufbeantworter) → Event 42 (hypothesis based on **623 internal number)
- Other Anrufbeantworter numbers may follow similar **6xx internal numbering

Report any discrepancies to help improve the mapping accuracy.
