# go-ewf CLI Tool Usage

A command-line tool for creating, dumping, and inspecting EWF (Expert Witness Format) forensic images.

## Installation

```bash
# Build the tool
go build -o ewf-tool ./cmd/main.go

# Or install globally
go install ./cmd/...
```

## Commands

### 1. Create - Create EWF Image from Raw Data

Convert raw disk images or files into EWF format with metadata.

**Basic Usage:**
```bash
ewf-tool create -source <input-file> -target <output-file> [options]
```

**Options:**
- `-source` (required): Source raw data file
- `-target` (required): Target EWF image file
- `-format`: EWF format - `evf1` (E01) or `evf2` (Ex01) (default: evf2)
- `-case-number`: Case number
- `-evidence-number`: Evidence number
- `-examiner`: Examiner name
- `-notes`: Notes about the evidence
- `-description`: Description
- `-drive-model`: Drive model (EVF2 only)
- `-serial-number`: Serial number (EVF2 only)
- `-buffer`: Buffer size in bytes (default: 1MB)
- `-verbose`: Show progress information

**Examples:**

Create an EVF2 (Ex01) image with metadata:
```bash
ewf-tool create \
  -source /dev/sda \
  -target /evidence/disk001 \
  -format evf2 \
  -case-number "CASE-2024-001" \
  -evidence-number "EVD-001" \
  -examiner "John Doe" \
  -notes "Suspect laptop hard drive" \
  -drive-model "Samsung SSD 850" \
  -serial-number "S2R5NX0K123456" \
  -verbose
```

Create a simple EVF1 (E01) image:
```bash
ewf-tool create \
  -source disk.raw \
  -target evidence.E01 \
  -format evf1 \
  -case-number "CASE-001" \
  -verbose
```

### 2. Dump - Extract Data from EWF Image

Extract raw data from EWF images, optionally specifying offset and length.

**Basic Usage:**
```bash
ewf-tool dump -source <ewf-file> -target <output-file> [options]
```

**Options:**
- `-source` (required): Source EWF image file
- `-target` (required): Target output file
- `-offset`: Start offset in bytes (default: 0)
- `-length`: Number of bytes to extract, -1 for all (default: -1)
- `-buffer`: Buffer size in bytes (default: 1MB)
- `-verbose`: Show progress information

**Examples:**

Extract entire image:
```bash
ewf-tool dump \
  -source evidence.Ex01 \
  -target recovered.raw \
  -verbose
```

Extract specific range (e.g., first 100MB):
```bash
ewf-tool dump \
  -source evidence.E01 \
  -target partition.raw \
  -offset 0 \
  -length 104857600 \
  -verbose
```

Extract from specific offset to end:
```bash
ewf-tool dump \
  -source evidence.Ex01 \
  -target data.raw \
  -offset 1048576 \
  -verbose
```

### 3. Info - Display Image Information

Show metadata and information about an EWF image.

**Basic Usage:**
```bash
ewf-tool info -source <ewf-file>
```

**Options:**
- `-source` (required): Source EWF image file

**Example:**
```bash
ewf-tool info -source evidence.Ex01
```

**Output Example:**
```
EWF Image Information
=====================

File: evidence.Ex01
Format: EVF2 (Ex01)
Size: 500107862016 bytes (465.76 GB)
Chunk Size: 32768 bytes

Metadata:

  Device Information:
    Drive Model: Samsung SSD 850
    Serial Number: S2R5NX0K123456
    Is physical: 1
    Drive Type: f
    Bytes per Sector: 512
    Number of Sectors: 976773168

  Case Data:
    Case Number: CASE-2024-001
    Evidence Number: EVD-001
    Examiner name: John Doe
    Notes: Suspect laptop hard drive
```

### 4. Version - Show Version Information

```bash
ewf-tool version
```

### 5. Help - Show Help Information

```bash
ewf-tool help
# or
ewf-tool <command> -h
```

## Format Comparison

### EVF1 (E01)
- **Extension**: `.E01`
- **Compression**: zlib only
- **Metadata**: Simple key-value pairs
- **Use case**: Older tools, broader compatibility

### EVF2 (Ex01)
- **Extension**: `.Ex01`
- **Compression**: zlib, bzip2
- **Metadata**: Structured sections (case data, device info)
- **Use case**: Modern tools, more detailed metadata

## Performance Tips

1. **Buffer Size**: Increase buffer size for large files
   ```bash
   ewf-tool create -source disk.raw -target disk.Ex01 -buffer 10485760  # 10MB buffer
   ```

2. **Use verbose mode** to monitor progress on large files
   ```bash
   ewf-tool dump -source large.Ex01 -target output.raw -verbose
   ```

3. **Partial extraction** is much faster than extracting full image
   ```bash
   ewf-tool dump -source disk.Ex01 -target mbr.raw -offset 0 -length 512
   ```

## Common Use Cases

### Convert DD to EWF

```bash
ewf-tool create \
  -source disk.dd \
  -target disk.Ex01 \
  -format evf2 \
  -case-number "$(date +%Y-%m-%d)" \
  -verbose
```

### Convert EWF to DD

```bash
ewf-tool dump \
  -source evidence.Ex01 \
  -target disk.dd \
  -verbose
```

### Extract MBR

```bash
ewf-tool dump \
  -source disk.E01 \
  -target mbr.bin \
  -offset 0 \
  -length 512
```

### Extract Partition

```bash
# Extract partition starting at offset 1048576 (1MB), size 10GB
ewf-tool dump \
  -source disk.Ex01 \
  -target partition1.raw \
  -offset 1048576 \
  -length 10737418240 \
  -verbose
```

### Inspect Image Before Processing

```bash
ewf-tool info -source unknown.E01
```

## Scripting Examples

### Batch Convert Multiple Images

```bash
#!/bin/bash
for file in *.dd; do
  base=$(basename "$file" .dd)
  echo "Converting $file..."
  ewf-tool create \
    -source "$file" \
    -target "${base}.Ex01" \
    -format evf2 \
    -case-number "BATCH-$(date +%Y%m%d)" \
    -verbose
done
```

### Verify Image Integrity

```bash
#!/bin/bash
# Extract and compare checksums
ewf-tool dump -source evidence.Ex01 -target /tmp/verify.raw
sha256sum /tmp/verify.raw
rm /tmp/verify.raw
```

## Exit Codes

- `0`: Success
- `1`: Error occurred (check stderr for details)

## Notes

- EWF images include padding to align with chunk boundaries (32KB by default)
- The reported size may be slightly larger than the original data
- Hash values (MD5, SHA1) stored in EWF include the padding
- When dumping, specify exact original size or use `-length` to avoid padding

## Troubleshooting

### "Failed to open as EVF1 or EVF2"
- File may be corrupted
- File may not be a valid EWF image
- Try running `info` command to check format

### "Offset beyond image size"
- Check image size first with `info` command
- Ensure offset is less than total image size

### Slow performance
- Increase buffer size with `-buffer`
- Use SSD for temporary storage
- Check available disk space

## Integration with Other Tools

### Mount EWF with FUSE

After extracting:
```bash
ewf-tool dump -source image.Ex01 -target image.raw
sudo losetup /dev/loop0 image.raw
sudo mount /dev/loop0p1 /mnt/evidence
```

### Use with Sleuth Kit

```bash
# Extract first
ewf-tool dump -source evidence.E01 -target evidence.raw

# Then analyze
fls evidence.raw
icat evidence.raw 5
```

### Use with Autopsy

Autopsy can read EWF files directly, but you can convert if needed:
```bash
ewf-tool dump -source case.Ex01 -target case.dd
```

## Support

For issues or questions, see the main README.md or open an issue on GitHub.

