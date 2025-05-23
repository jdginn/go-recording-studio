package opc

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// CompressionOption is an enumerable for the different compression options.
type CompressionOption int

const (
	// CompressionNone disables the compression.
	CompressionNone CompressionOption = iota - 1
	// CompressionNormal is optimized for a reasonable compromise between size and performance.
	CompressionNormal
	// CompressionMaximum is optimized for size.
	CompressionMaximum
	// CompressionFast is optimized for performance.
	CompressionFast
	// CompressionSuperFast is optimized for super performance.
	CompressionSuperFast
)

// Writer implements a OPC file writer.
type Writer struct {
	Properties    CoreProperties  // Package metadata. Can be modified until the Writer is closed.
	Relationships []*Relationship // The relationships associated to the package. Can be modified until the Writer is closed.
	p             *pkg
	w             *zip.Writer
	last          *Part
}

// NewWriter returns a new Writer writing an OPC package to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{p: &pkg{
		parts: make(map[string]struct{}, 0),
		contentTypes: contentTypes{
			defaults: map[string]string{
				"rels": relationshipContentType,
			},
			overrides: map[string]string{},
		},
	}, w: zip.NewWriter(w)}
}

// NewWriterFromReader returns a new Writer writing an OPC package to w
// and with its content initialized with r.
//
// The original package is not modified.
// Parts coming from r cannot be modified but new parts can be appended
// and package core properties and relationships can be updated.
func NewWriterFromReader(w io.Writer, r *Reader) (*Writer, error) {
	ow := NewWriter(w)
	for _, p := range r.Files {
		pw, err := ow.CreatePart(p.Part, CompressionNormal)
		if err != nil {
			return nil, err
		}
		rc, err := p.Open()
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(pw, rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
	}
	ow.Properties = r.Properties
	ow.Relationships = make([]*Relationship, len(r.Relationships))
	for i, rel := range r.Relationships {
		ow.Relationships[i] = &(*rel)
	}
	return ow, nil
}

// Flush flushes any buffered data to the underlying writer.
// Part metadata, relationships, content types and other OPC related files won't be flushed.
// Calling Flush is not normally necessary; calling Close is sufficient.
// Useful to do simultaneous writing and reading.
func (w *Writer) Flush() error {
	return w.w.Flush()
}

// Close finishes writing the opc file.
// It does not close the underlying writer.
func (w *Writer) Close() error {
	if err := w.createLastPartRelationships(); err != nil {
		w.w.Close()
		return err
	}
	if err := w.createCoreProperties(); err != nil {
		w.w.Close()
		return err
	}
	if err := w.createOwnRelationships(); err != nil {
		w.w.Close()
		return err
	}
	if err := w.createContentTypes(); err != nil {
		w.w.Close()
		return err
	}
	return w.w.Close()
}

// Create adds a file to the OPC archive using the provided name and content type.
// The file contents will be compressed using the Deflate default method.
// The name shall be a valid part name, one can use NormalizePartName before calling Create to normalize it
//
// This returns a Writer to which the file contents should be written.
// The file's contents must be written to the io.Writer before the next call to Create, CreatePart, or Close.
func (w *Writer) Create(name, contentType string) (io.Writer, error) {
	return w.CreatePart(&Part{Name: name, ContentType: contentType}, CompressionNone)
}

// CreatePart adds a file to the OPC archive using the provided part.
// The name shall be a valid part name, one can use NormalizePartName before calling CreatePart to normalize it.
// Writer takes ownership of part and may mutate all its fields except the Relationships,
// which can be modified until the next call to Create, CreatePart or Close.
// The caller must not modify part after calling CreatePart, except the Relationships.
//
// This returns a Writer to which the file contents should be written.
// The file's contents must be written to the io.Writer before the next call to Create, CreatePart, or Close.
func (w *Writer) CreatePart(part *Part, compression CompressionOption) (io.Writer, error) {
	return w.add(part, compression)
}

func (w *Writer) createCoreProperties() error {
	if w.Properties == (CoreProperties{}) {
		return nil
	}
	partName := w.Properties.PartName
	if partName == "" {
		partName = corePropsDefaultName
	}
	part := &Part{Name: partName, ContentType: corePropsContentType}
	if !strings.HasPrefix(partName, "/") {
		part.Name = "/" + partName
	}
	cw, err := w.addToPackage(part, CompressionNormal)
	if err != nil {
		return err
	}
	var hasCoreRel bool
	for _, rel := range w.Relationships {
		if strings.EqualFold(rel.Type, corePropsRel) {
			hasCoreRel = true
			break
		}
	}
	if !hasCoreRel {
		w.Relationships = append(w.Relationships, &Relationship{
			w.Properties.RelationshipID, corePropsRel, partName, ModeInternal,
		})
	}
	return w.Properties.encode(cw)
}

func (w *Writer) createContentTypes() error {
	// ISO/IEC 29500-2 M3.10
	fh := &zip.FileHeader{
		Name:     zipName(contentTypesName),
		Modified: time.Now(),
	}
	w.setCompressor(fh, CompressionNormal)
	cw, err := w.w.CreateHeader(fh)
	if err != nil {
		return err
	}
	return w.p.encodeContentTypes(cw)
}

func (w *Writer) createOwnRelationships() error {
	if len(w.Relationships) == 0 {
		return nil
	}
	for _, r := range w.Relationships {
		if r.ID == "" {
			r.ID = newRelationshipID(w.Relationships)
		}
	}
	if err := validateRelationships("/", w.Relationships); err != nil {
		return err
	}
	rw, err := w.addToPackage(&Part{Name: packageRelName, ContentType: relationshipContentType}, CompressionNormal)
	if err != nil {
		return err
	}
	return encodeRelationships(rw, w.Relationships)
}

func (w *Writer) createLastPartRelationships() error {
	if w.last == nil || len(w.last.Relationships) == 0 {
		return nil
	}
	for _, r := range w.last.Relationships {
		if r.ID == "" {
			r.ID = newRelationshipID(w.last.Relationships)
		}
	}
	if err := validateRelationships(w.last.Name, w.last.Relationships); err != nil {
		return err
	}
	dirName := path.Dir(w.last.Name)[1:]
	if dirName != "" {
		dirName = "/" + dirName
	}
	relName := fmt.Sprintf("%s/_rels/%s.rels", dirName, path.Base(w.last.Name))
	rw, err := w.addToPackage(&Part{Name: relName, ContentType: relationshipContentType}, CompressionNormal)
	if err != nil {
		return err
	}
	return encodeRelationships(rw, w.last.Relationships)
}

func (w *Writer) add(part *Part, compression CompressionOption) (io.Writer, error) {
	if err := w.createLastPartRelationships(); err != nil {
		return nil, err
	}
	pw, err := w.addToPackage(part, compression)
	if err == nil {
		w.last = part
	}
	return pw, err
}

func (w *Writer) addToPackage(part *Part, compression CompressionOption) (io.Writer, error) {
	// Validate name and check for duplicated names ISO/IEC 29500-2 M3.3
	if err := w.p.add(part); err != nil {
		return nil, err
	}
	fh := &zip.FileHeader{
		Name:     zipName(part.Name),
		Modified: time.Now(),
	}
	w.setCompressor(fh, compression)
	pw, err := w.w.CreateHeader(fh)
	if err != nil {
		w.p.deletePart(part.Name)
		return nil, fmt.Errorf("opc: %s: cannot be created: %v", part.Name, err)
	}
	return pw, nil
}

func (w *Writer) setCompressor(fh *zip.FileHeader, compression CompressionOption) {
	var comp int
	switch compression {
	case CompressionNormal:
		comp = flate.DefaultCompression
	case CompressionMaximum:
		comp = flate.BestCompression
		fh.Flags |= 0x2
	case CompressionFast:
		comp = flate.BestSpeed
		fh.Flags |= 0x4
	case CompressionSuperFast:
		comp = flate.BestSpeed
		fh.Flags |= 0x6
	case CompressionNone:
		comp = flate.NoCompression
	default:
		comp = -1000 // write will failt
	}

	fh.Method = zip.Deflate
	w.w.RegisterCompressor(zip.Deflate, compressionFunc(comp))
}

func compressionFunc(comp int) func(out io.Writer) (io.WriteCloser, error) {
	return func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, comp)
	}
}

func zipName(partName string) string {
	// ISO/IEC 29500-2 M3.4
	return partName[1:] // remove first slash
}
