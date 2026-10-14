# vconnect bootstrap - loaded once per session via "source" after Vivado starts
# This file is owned by vconnect; do not edit by hand.
#
# vconn_exec <id> <tmpfile> <script>
#
#   Runs <script> under [catch].
#   Writes a JSON object to <tmpfile>:
#     { "ok": true,  "value": <string> }
#     { "ok": false, "error": <string>, "errorCode": <string> }
#   Then prints exactly the sentinel line:
#     VCONN_DONE <id>
#
proc vconn_exec {id tmpfile script} {
    set rc [catch {uplevel #0 $script} result opts]
    if {$rc == 0} {
        set json [vconn_json_obj [list ok true value $result]]
    } else {
        set ec ""
        catch {set ec [dict get $opts -errorcode]}
        set json [vconn_json_obj [list ok false error $result errorCode $ec]]
    }
    # atomic write: write to tmp.part, then rename
    set part "${tmpfile}.part"
    set fh [open $part w]
    puts -nonewline $fh $json
    close $fh
    file rename -force $part $tmpfile
    # sentinel — only thing we watch for on stdout
    puts "VCONN_DONE $id"
    flush stdout
}

# Minimal JSON serialiser (no deps).
# Handles: bool, null, numbers, strings (with escaping), lists-as-arrays,
#           dicts-as-objects (alternating key/value list).
proc vconn_to_json {val} {
    # booleans passed as Tcl "true"/"false" literals
    if {$val eq "true" || $val eq "false" || $val eq "null"} {
        return $val
    }
    # number?
    if {[string is double -strict $val] || [string is integer -strict $val]} {
        return $val
    }
    # string — escape JSON special chars
    set s $val
    set s [string map {\\ \\\\ \" \\\" \n \\n \r \\r \t \\t} $s]
    return "\"$s\""
}

proc vconn_json_arr {lst} {
    set parts {}
    foreach item $lst {
        lappend parts [vconn_to_json $item]
    }
    return "\[[join $parts ,]\]"
}

proc vconn_json_obj {kvlist} {
    set parts {}
    foreach {k v} $kvlist {
        lappend parts "\"$k\":[vconn_to_json $v]"
    }
    return "{[join $parts ,]}"
}

# Parse the Vivado message stream from a string (used by run_tcl result).
# Returns a Tcl list of dicts: {severity id text}
proc vconn_parse_messages {text} {
    set msgs {}
    foreach line [split $text \n] {
        # Vivado format: INFO: [Synth 8-327] message text
        if {[regexp {^(INFO|WARNING|CRITICAL WARNING|ERROR):\s+\[([^\]]+)\]\s+(.*)} \
                $line -> sev id msg]} {
            lappend msgs [list severity $sev id $id text $msg]
        }
    }
    return $msgs
}

# Expose Vivado version info
proc vconn_version {} {
    set v ""
    catch {set v [version -short]}
    return $v
}

# Print ready sentinel so the Go side knows init is done
puts "VCONN_READY"
flush stdout
