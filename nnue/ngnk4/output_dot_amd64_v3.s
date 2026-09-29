//go:build amd64 && amd64.v3

#include "go_asm.h"
#include "textflag.h"

// k4OutputDotAVX2 computes the exact modular int32 output sum. For each
// clipped lane v, v^2 is split into the two signed-int16-safe products
// v*floor(v/2) and v*ceil(v/2). PMADDWD multiplies those by duplicated signed
// weights and reconstructs v^2*w exactly, including w=-32768. All vector and
// horizontal additions wrap modulo 2^32, matching Go int32 addition.
TEXT ·k4OutputDotAVX2(SB), NOSPLIT, $0-36
	MOVQ stm+0(FP), AX
	MOVQ nonSTM+8(FP), BX
	MOVQ stmWeights+16(FP), CX
	MOVQ nonSTMWeights+24(FP), DX

	VPXOR Y12, Y12, Y12
	VPXOR Y15, Y15, Y15
	MOVL $0x00ff00ff, R8
	VMOVD R8, X14
	VPBROADCASTD X14, Y14
	MOVL $0x00010001, R8
	VMOVD R8, X13
	VPBROADCASTD X13, Y13

	MOVL $const_k4LaneBlocks, R9
k4_output_block:
	VMOVDQU (AX), Y0
	VPMAXSW Y15, Y0, Y0
	VPMINSW Y14, Y0, Y0
	VPSRLW $1, Y0, Y1
	VPAND Y13, Y0, Y2
	VPADDW Y1, Y2, Y2
	VPMULLW Y0, Y1, Y1
	VPMULLW Y0, Y2, Y2
	VPUNPCKLWD Y2, Y1, Y3
	VPUNPCKHWD Y2, Y1, Y4
	VMOVDQU (CX), Y5
	VPUNPCKLWD Y5, Y5, Y6
	VPUNPCKHWD Y5, Y5, Y7
	VPMADDWD Y6, Y3, Y3
	VPMADDWD Y7, Y4, Y4
	VPADDD Y3, Y12, Y12
	VPADDD Y4, Y12, Y12

	VMOVDQU (BX), Y0
	VPMAXSW Y15, Y0, Y0
	VPMINSW Y14, Y0, Y0
	VPSRLW $1, Y0, Y1
	VPAND Y13, Y0, Y2
	VPADDW Y1, Y2, Y2
	VPMULLW Y0, Y1, Y1
	VPMULLW Y0, Y2, Y2
	VPUNPCKLWD Y2, Y1, Y3
	VPUNPCKHWD Y2, Y1, Y4
	VMOVDQU (DX), Y5
	VPUNPCKLWD Y5, Y5, Y6
	VPUNPCKHWD Y5, Y5, Y7
	VPMADDWD Y6, Y3, Y3
	VPMADDWD Y7, Y4, Y4
	VPADDD Y3, Y12, Y12
	VPADDD Y4, Y12, Y12

	ADDQ $32, AX
	ADDQ $32, BX
	ADDQ $32, CX
	ADDQ $32, DX
	DECL R9
	JNZ k4_output_block

	VEXTRACTI128 $1, Y12, X0
	VPADDD X0, X12, X12
	VPHADDD X12, X12, X12
	VPHADDD X12, X12, X12
	VMOVD X12, R8
	MOVL R8, ret+32(FP)
	VZEROUPPER
	RET
