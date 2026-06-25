#!/bin/bash
BASE="/home/mrafter/nas-os/internal"
MERGED=0

merge_group() {
    local target="$1"
    shift
    local sources=("$@")
    
    echo "━━━ Merging into $target ━━━"
    mkdir -p "$BASE/$target"
    
    for src in "${sources[@]}"; do
        if [ ! -d "$BASE/$src" ]; then
            echo "  SKIP $src (not found)"
            continue
        fi
        
        local gofiles=$(find "$BASE/$src" -maxdepth 1 -name "*.go" ! -name "*_test.go" 2>/dev/null)
        if [ -z "$gofiles" ]; then
            echo "  SKIP $src (no go files)"
            rm -rf "$BASE/$src" 2>/dev/null || true
            continue
        fi
        
        local count=0
        while IFS= read -r f; do
            [ -z "$f" ] && continue
            local bname=$(basename "$f")
            local dest="$BASE/$target/${src}__${bname}"
            cp "$f" "$dest"
            # Fix package declaration
            sed -i "1,5s/^package [a-zA-Z0-9_]*/package $target/" "$dest"
            count=$((count + 1))
        done <<< "$gofiles"
        
        rm -rf "$BASE/$src"
        echo "  OK $src -> $target ($count files)"
        MERGED=$((MERGED + 1))
    done
}

# 1. Ransomware
merge_group "ransomware" \
    ransomwarecanary ransomware_defense ransomwareguard ransomware_honeypot \
    ransomai ransombehaviorai ransomhoneypot ransomml2 ransommldetect \
    ransomshield antiRansomHoneypot behaviorransom

# 2. Compliance
merge_group "compliance" \
    compliance1403 complianceaudit complianceauto complianceautomation \
    compliancecenter compliancechecker compliancedash \
    complianceengine compliancereport compliancescan compliancescanner \
    compliancetracker smartcompliance

# 3. Tiering
merge_group "tiering" \
    smarttier smarttier2 smarttierengine smarttiering \
    smartdatatier smartdatatiering storagetiering adaptivetier \
    datatiering datatierml tierrules

# 4. Photo AI
merge_group "photoai" \
    aiphoto aiphotoalbum aiphotoenhanced aiphotomanager smartphoto

# 5. Cost Analyzer
merge_group "costanalyzer" \
    costanalysis costoptimizer costoptimizer2 smartcostoptimizer \
    storagecostoptimizer cloudcostopt

# 6. Disk Health
merge_group "diskhealth" \
    diskhealthai diskhealthai2 aidiskhealth smarthealth \
    smarthealthpredict smarthealthscore

# 7. Backup
merge_group "backup" \
    smartbackup smartbackupadvisor smartbackuporch smartbackupsched \
    smartbackupverify activebackup backupconsole backupdedup \
    backupencrypt backupvault backupverify

# 8. AI Console
merge_group "aiconsole" \
    aiconsole2 aiconsoledatamask

# 9. AI Agent
merge_group "aiagentorch" \
    aiagentorchestrator

echo ""
echo "Done. Merged $MERGED modules."
