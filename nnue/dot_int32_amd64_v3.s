//go:build amd64 && amd64.v3

#include "textflag.h"

// boundedOutputDotAVX2 evaluates two 128-lane accumulator halves. Eight int32
// activations are clipped at a time, then their 128-bit halves are packed in
// order to avoid AVX2's lane-local 256-bit PACKSSDW permutation. Since
// activation*weight fits int16 for weights in [-128,128], PMADDWD computes
// activation*(activation*weight) exactly in signed int32 pairs.
TEXT ·boundedOutputDotAVX2(SB), NOSPLIT, $0-28
	MOVQ us+0(FP), AX
	MOVQ them+8(FP), BX
	MOVQ weights+16(FP), CX

	VPXOR X0, X0, X0
	MOVL $255, DX
	VMOVD DX, X1
	VPBROADCASTD X1, Y1
	VPXOR X7, X7, X7

	MOVL $16, DX
us_loop:
	VMOVDQU (AX), Y2
	VPMAXSD Y0, Y2, Y2
	VPMINSD Y1, Y2, Y2
	VEXTRACTI128 $1, Y2, X3
	VPACKSSDW X3, X2, X2
	VMOVDQU (CX), X4
	VPMULLW X4, X2, X5
	VPMADDWD X5, X2, X6
	VPADDD X6, X7, X7
	ADDQ $32, AX
	ADDQ $16, CX
	DECL DX
	JNZ us_loop

	MOVL $16, DX
them_loop:
	VMOVDQU (BX), Y2
	VPMAXSD Y0, Y2, Y2
	VPMINSD Y1, Y2, Y2
	VEXTRACTI128 $1, Y2, X3
	VPACKSSDW X3, X2, X2
	VMOVDQU (CX), X4
	VPMULLW X4, X2, X5
	VPMADDWD X5, X2, X6
	VPADDD X6, X7, X7
	ADDQ $32, BX
	ADDQ $16, CX
	DECL DX
	JNZ them_loop

	VPHADDD X7, X7, X7
	VPHADDD X7, X7, X7
	VMOVD X7, AX
	MOVL AX, ret+24(FP)
	VZEROUPPER
	RET
